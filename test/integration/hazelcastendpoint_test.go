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
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
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
	}

	expectAddressesInHazelcastEndpointsMatchWithServices := func(hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint, services []corev1.Service) {
		for _, hzEndpoint := range hzEndpoints {
			By(fmt.Sprintf("matching the '%s' hazelcastEndpoint to its corresponding service", hzEndpoint.Name))
			var service *corev1.Service
			for _, svc := range services {
				switch hzEndpoint.Spec.Type {
				case hazelcastv1alpha1.HazelcastEndpointTypeDiscovery, hazelcastv1alpha1.HazelcastEndpointTypeMember:
					if hzEndpoint.Name == svc.Name {
						service = &svc
					}
				case hazelcastv1alpha1.HazelcastEndpointTypeWAN:
					if strings.HasPrefix(hzEndpoint.Name, svc.Name+"-wan") {
						service = &svc
					}
				}
				if service != nil {
					break
				}
			}
			Expect(service).NotTo(BeNil())
			Expect(service.Spec.Type).Should(Not(Equal(corev1.ServiceTypeClusterIP)))

			hzEndpointIpAddr, hzEndpointPortStr, err := net.SplitHostPort(hzEndpoint.Status.Address)
			Expect(err).To(BeNil())
			hzEndpointPort, err := strconv.Atoi(hzEndpointPortStr)
			Expect(err).To(BeNil())

			By("comparing the hazelcastEndpoint's address with the service's external IP address")
			switch service.Spec.Type {
			case corev1.ServiceTypeLoadBalancer:
				serviceExternalAddress := util.GetExternalAddress(service)
				Expect(hzEndpointIpAddr).Should(ContainSubstring(serviceExternalAddress))
			case corev1.ServiceTypeNodePort:
				Expect(hzEndpointIpAddr).Should(Equal("*"))
			}

			By("checking if the hazelcastEndpoint's port is exposed on the service")
			var ports []int32
			for _, port := range service.Spec.Ports {
				switch service.Spec.Type {
				case corev1.ServiceTypeLoadBalancer:
					ports = append(ports, port.Port)
				case corev1.ServiceTypeNodePort:
					ports = append(ports, port.NodePort)
				}
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
			}, timeout, interval).Should(
				Or(
					MatchRegexp(`^((25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d):([1-9]\d{0,3}|[1-5]\d{4}|6[0-4]\d{3}|65[0-4]\d{2}|655[0-2]\d|6553[0-5])$`),
					MatchRegexp(`^\*:(\d{1,5})$`),
				))
		}
	}

	expectLenOfHazelcastServicesWithHazelcastEndpointLabel := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, expectedLen int) []corev1.Service {
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
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("Hazelcast which doesn't expose any external endpoints", func() {
		It("should not create HazelcastEndpoints", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}
			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 0)
			expectLenOfHazelcastEndpoints(ctx, hz, 0)
		})
	})

	Context("Hazelcast that exposes external endpoints", func() {
		When("creating Hazelcast with expose externally enabled", func() {
			It("should create HazelcastEndpoint with Discovery type when Hazelcast is exposed via Unisocket and LoadBalancer", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 1)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 2)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)

				expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})

			It("should create HazelcastEndpoint with Discovery type when Hazelcast is exposed via Unisocket and NodePort", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 1)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 2)

				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)

				expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})

			It("should create HazelcastEndpoint with Discovery and Member type when per member is exposed using LoadBalancer", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 4)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)

				expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})

			It("should create HazelcastEndpoint with Discovery and Member type when per member is exposed using NodePort", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}
				hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 4)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)

				expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})
		})

		When("creating Hazelcast with WAN", func() {
			It("should create HazelcastEndpoint with WAN type", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}

				hz.Spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
					WAN: []hazelcastv1alpha1.WANConfig{
						{
							Name:        "florida",
							Port:        5710,
							PortCount:   9,
							ServiceType: hazelcastv1alpha1.WANServiceTypeLoadBalancer,
						},
						{
							Name:        "ottawa",
							Port:        5720,
							PortCount:   4,
							ServiceType: hazelcastv1alpha1.WANServiceTypeNodePort,
						},
						{
							Name:        "oslo",
							Port:        5730,
							PortCount:   4,
							ServiceType: hazelcastv1alpha1.WANServiceTypeClusterIP,
						},
					},
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 2)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 13)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 20*time.Second)

				expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
			})
		})

		It("should create HazelcastEndpoint for WAN WithExposeExternally service type", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}
			hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz.Spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Name:        "florida",
						Port:        5710,
						PortCount:   3,
						ServiceType: hazelcastv1alpha1.WANServiceTypeWithExposeExternally,
					},
					{
						Name:        "ottawa",
						Port:        5720,
						PortCount:   2,
						ServiceType: hazelcastv1alpha1.WANServiceTypeWithExposeExternally,
					},
				},
			}

			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 4)
			hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 24)

			setLoadBalancerIngressAddress(ctx, services)
			expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 20*time.Second)

			expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)
		})

	})

	When("Hazelcast is updated in a way that the exposeExternally is disabled", func() {
		It("should delete the leftover resources", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}
			hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}

			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 4)
			hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

			setLoadBalancerIngressAddress(ctx, services)
			expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)

			expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)

			By("updating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Get(context.Background(), lookupKey(hz), hz)).Should(Succeed())
			hz.Spec.ExposeExternally = nil
			Expect(k8sClient.Update(context.Background(), hz)).Should(Succeed())

			expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 0)
			expectLenOfHazelcastEndpoints(ctx, hz, 0)
		})
	})

	When("HazelcastEndpoint spec is updated", func() {
		It("should fail", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}
			hz.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}

			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			services := expectLenOfHazelcastServicesWithHazelcastEndpointLabel(ctx, hz, 4)
			hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 5)

			setLoadBalancerIngressAddress(ctx, services)
			expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 10*time.Second)

			expectAddressesInHazelcastEndpointsMatchWithServices(hzEndpoints, services)

			hzEndpoint := hzEndpoints[0]
			hzEndpoint.Spec.Port = 5790
			err := k8sClient.Update(ctx, &hzEndpoint)
			Expect(err).Should(MatchError(ContainSubstring("field cannot be updated")))
		})
	})
})
