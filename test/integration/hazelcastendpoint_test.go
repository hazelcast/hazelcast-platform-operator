package integration

import (
	"context"
	"fmt"
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

var _ = FDescribe("HazelcastEndpoint CR", func() {
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

	matchHazelcastEndpointsWithServices := func(hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint, services []corev1.Service) {
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

			addr := util.GetExternalAddress(service)
			fmt.Printf("Addre: %s\n", addr)
			Expect(hzEndpoint.Status.Address).Should(Equal(addr))
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

	expectLenOfHazelcastEndpointServices := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, expectedLen int) []corev1.Service {
		var services []corev1.Service
		Eventually(func() []corev1.Service {
			svcList, err := util.ListRelatedEndpointServices(ctx, k8sClient, hz)
			Expect(err).ToNot(HaveOccurred())
			services = svcList.Items
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
		DeleteAllOf(&hazelcastv1alpha1.HazelcastEndpoint{}, &hazelcastv1alpha1.HazelcastEndpointList{}, namespace, map[string]string{})
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
			It("should create HazelcastEndpoint in Discovery type", Label("fast"), func() {
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

				services := expectLenOfHazelcastEndpointServices(ctx, hz, 1)
				hzEndpoints := expectLenOfHazelcastEndpoints(ctx, hz, 1)

				setLoadBalancerIngressAddress(ctx, services)
				expectHazelcastEndpointHasAddress(ctx, hzEndpoints, 30*time.Second)
				matchHazelcastEndpointsWithServices(hzEndpoints, services)
			})

			/*
				It("should create HazelcastEndpoint with Discovery and Member type", Label("fast"), func() {
					lbCtx, cancel := context.WithCancel(ctx)
					go AssignLoadBalancerAddress(lbCtx, k8sClient, Second, client.InNamespace(namespace))
					defer cancel()

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

					svc := getHazelcastService(ctx, hz, Minute)
					hzEndpoints := listHazelcastEndpoints(ctx, hz, Minute, 4)

					waitForServiceToBeAssignedLoadBalancerAddress(ctx, svc, 20*Second)
					waitForHazelcastEndpointToHaveAddress(ctx, hzEndpoints[0], 20*Second)

					memberServices := getHazelcastMemberServices(ctx, hz, Minute)
					for _, memberSvc := range memberServices {
						waitForServiceToBeAssignedLoadBalancerAddress(ctx, &memberSvc, 20*Second)
					}

					for _, hzEndpoint := range hzEndpoints {
						waitForHazelcastEndpointToHaveAddress(ctx, hzEndpoint, 20*Second)
					}

					hzep, ok := FindResourceByName(hzEndpoints, svc.Name)
					Expect(ok).To(BeTrue())
					expectHazelcastEndpointMatchedWithService(hzep, svc)
					Expect(hzep.Spec.Type).Should(Equal(hazelcastv1alpha1.HazelcastEndpointTypeDiscovery))

					for _, memberSvc := range memberServices {
						hzep, ok := FindResourceByName(hzEndpoints, memberSvc.Name)
						Expect(ok).To(BeTrue())
						expectHazelcastEndpointMatchedWithService(hzep, &memberSvc)
						Expect(hzep.Spec.Type).Should(Equal(hazelcastv1alpha1.HazelcastEndpointTypeMember))
					}
				})

			*/
		})

		/*
			When("creating Hazelcast with WAN", func() {
				It("should create HazelcastEndpoint in WAN type", Label("fast"), func() {
					lbCtx, cancel := context.WithCancel(ctx)
					go AssignLoadBalancerAddress(lbCtx, k8sClient, Second, client.InNamespace(namespace))
					defer cancel()

					hz := &hazelcastv1alpha1.Hazelcast{
						ObjectMeta: randomObjectMeta(namespace),
						Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
					}

					hz.Spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
						WAN: []hazelcastv1alpha1.WANConfig{
							{
								Name:        "miami",
								Port:        5710,
								PortCount:   3,
								ServiceType: corev1.ServiceTypeLoadBalancer,
							},
							{
								Name:        "new-york-city",
								Port:        5720,
								PortCount:   5,
								ServiceType: corev1.ServiceTypeLoadBalancer,
							},
						},
					}

					By("creating the Hazelcast CR with specs successfully")
					Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

					wanServices := getWanServices(ctx, hz, Minute)
					hzEndpoints := listHazelcastEndpoints(ctx, hz, Minute, 8)

					for _, wanSvc := range wanServices {
						waitForServiceToBeAssignedLoadBalancerAddress(ctx, wanSvc, 20*Second)
					}

					for _, hzep := range hzEndpoints {
						waitForHazelcastEndpointToHaveAddress(ctx, hzep, 20*Second)
					}

					for _, wan := range hz.Spec.AdvancedNetwork.WAN {
						for i := 0; i < int(wan.PortCount); i++ {
							svcName := fmt.Sprintf("%s-%s", hz.GetName(), wan.Name)
							svc, ok := FindResourceByName(wanServices, svcName)
							Expect(ok).To(BeTrue())
							hzepName := fmt.Sprintf("%s-%d", svcName, i)
							hzep, ok := FindResourceByName(hzEndpoints, hzepName)
							Expect(ok).To(BeTrue())
							expectHazelcastEndpointMatchedWithService(hzep, svc)
							Expect(hzep.Spec.Type).Should(Equal(hazelcastv1alpha1.HazelcastEndpointTypeWAN))
						}
					}
				})


			})
		*/
	})
})
