package integration

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("HazelcastEndpoint CR", func() {
	const namespace = "default"

	listHazelcastEndpoints := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast) []hazelcastv1alpha1.HazelcastEndpoint {
		hzEndpointList := hazelcastv1alpha1.HazelcastEndpointList{}
		nsOpt := client.InNamespace(hz.Namespace)
		lblOpt := client.MatchingLabels(util.Labels(hz))
		err := k8sClient.List(ctx, &hzEndpointList, nsOpt, lblOpt)
		Expect(err).ToNot(HaveOccurred())
		return hzEndpointList.Items
	}

	setLoadBalancerIngressAddress := func(ctx context.Context, services []corev1.Service) {
		for i := 0; i < len(services); i++ {
			svc := &services[i]
			svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{
					IP: RandomIpAddress(),
					Ports: []corev1.PortStatus{
						{
							Port:     svc.Spec.Ports[0].Port,
							Protocol: svc.Spec.Ports[0].Protocol,
						},
					},
				},
			}
			err := k8sClient.Status().Update(ctx, svc)
			Expect(err).ToNot(HaveOccurred())
		}
	}

	expectAddressInHazelcastEndpointsMatchWithServices := func(hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint, services []corev1.Service) {
		for _, hzEndpoint := range hzEndpoints {
			svcName := hzEndpoint.Name
			if hzEndpoint.Spec.Type == hazelcastv1alpha1.HazelcastEndpointTypeWAN {
				i := strings.LastIndex(hzEndpoint.Name, "-")
				svcName = hzEndpoint.Name[:i]
			}

			var service *corev1.Service
			for _, svc := range services {
				if svc.Name == svcName {
					service = &svc
					break
				}
			}
			Expect(service).NotTo(BeNil())

			if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
				continue
			}

			addr := util.GetExternalAddress(service)
			Expect(hzEndpoint.Status.Address).Should(ContainSubstring(addr))

			i := strings.Index(hzEndpoint.Status.Address, ":")
			hzEndpointPortStr := hzEndpoint.Status.Address[i+1:]
			hzEndpointPort, err := strconv.Atoi(hzEndpointPortStr)
			Expect(err).ToNot(HaveOccurred())
			var ports []int32
			for _, port := range service.Spec.Ports {
				ports = append(ports, port.Port)
			}
			Expect(ports).Should(ContainElement(int32(hzEndpointPort)))
		}
	}

	expectHazelcastEndpointHasAddress := func(ctx context.Context, hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint, timeout time.Duration) {
		for i := 0; i < len(hzEndpoints); i++ {
			hzEndpoint := &hzEndpoints[i]
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hzEndpoint.Namespace, Name: hzEndpoint.Name}, hzEndpoint)
				Expect(err).ToNot(HaveOccurred())
				return hzEndpoint.Status.Address
			}, timeout, interval).Should(Not(BeEmpty()))
		}
	}

	expectHazelcastEndpointHasNodeAddress := func(ctx context.Context, hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint, timeout time.Duration) {
		for i := 0; i < len(hzEndpoints); i++ {
			hzEndpoint := &hzEndpoints[i]
			Eventually(func() int {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hzEndpoint.Namespace, Name: hzEndpoint.Name}, hzEndpoint)
				Expect(err).ToNot(HaveOccurred())
				_, port, err := net.SplitHostPort(hzEndpoint.Status.Address)
				Expect(err).ToNot(HaveOccurred())
				endpointPort, err := strconv.Atoi(port)
				Expect(err).ToNot(HaveOccurred())
				return endpointPort
			}, timeout, interval).Should(BeNumerically(">=", 30000))
		}
	}

	expectLenOfHazelcastEndpointServices := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, expectedLen int) []corev1.Service {
		var services []corev1.Service
		Eventually(func() []corev1.Service {
			svcList, err := util.ListRelatedServices(ctx, k8sClient, hz)
			Expect(err).ToNot(HaveOccurred())
			services = []corev1.Service{}
			for _, svc := range svcList.Items {
				if _, ok := svc.Labels[n.ServiceEndpointTypeLabelName]; ok {
					services = append(services, svc)
				}
			}
			return services
		}, timeout, interval).Should(HaveLen(expectedLen))

		return services
	}

	expectLenOfHazelcastEndpoints := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, expectedLen int) []hazelcastv1alpha1.HazelcastEndpoint {
		var hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint
		Eventually(func() []hazelcastv1alpha1.HazelcastEndpoint {
			hzEndpoints = listHazelcastEndpoints(ctx, hz)
			return hzEndpoints
		}, timeout, interval).Should(HaveLen(expectedLen))

		return hzEndpoints
	}

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("Hazelcast which doesn't expose any external endpoints", func() {
		It("should not create HazelcastEndpoints", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			expectLenOfHazelcastEndpointServices(ctx, hz, 0)
			expectLenOfHazelcastEndpoints(ctx, hz, 0)
		})
	})

	Context("Hazelcast that exposes external endpoints", func() {
		When("creating Hazelcast with expose externally enabled", func() {
			It("should create HazelcastEndpoint with Discovery type when Hazelcast is exposed via Unisocket and LoadBalancer", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 1)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 2)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)
				expectAddressInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})

			It("should create HazelcastEndpoint with Discovery type when Hazelcast is exposed via Unisocket and NodePort", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 1)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 2)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)
				expectHazelcastEndpointHasNodeAddress(ctx, hzEndpoints, 10*time.Second)
			})

			It("should create HazelcastEndpoint with Discovery type when Hazelcast is exposed via Smart", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 4)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)
				expectAddressInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})

			It("should create HazelcastEndpoint with Discovery and Member type when per member is exposed using LoadBalancer", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 4)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)
				expectAddressInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})

			It("should create HazelcastEndpoint with Discovery and Member type when per member is exposed using NodePort", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 4)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)
				expectHazelcastEndpointHasNodeAddress(ctx, hzEndpoints, 10*time.Second)
			})
		})

		When("creating Hazelcast with WAN", func() {
			It("should create HazelcastEndpoint with WAN type", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
				}

				hz.Spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
					WAN: []hazelcastv1alpha1.WANConfig{
						{
							Name:        "florida",
							Port:        5710,
							PortCount:   3,
							ServiceType: corev1.ServiceTypeLoadBalancer,
						},
						{
							Name:        "ottawa",
							Port:        5714,
							PortCount:   5,
							ServiceType: corev1.ServiceTypeLoadBalancer,
						},
						{
							Name:        "oslo",
							Port:        5719,
							PortCount:   9,
							ServiceType: corev1.ServiceTypeClusterIP,
						},
					},
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 2)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 8)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 20*time.Second)
				expectAddressInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})
		})
	})

	When("HazelcastEndpoint spec is updated", func() {
		It("should fail", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}

			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			services := expectLenOfHazelcastEndpointServices(ctx, hz, 4)
			hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

			setLoadBalancerIngressAddress(ctx, services)
			expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)
			expectAddressInHazelcastEndpointsMatchWithServices(hzEndpoints, services)

			hzEndpoint := hzEndpoints[0]
			hzEndpoint.Spec.Port = 5790
			err := k8sClient.Update(ctx, &hzEndpoint)
			Expect(err).Should(MatchError(ContainSubstring("field cannot be updated")))
		})
	})
})
