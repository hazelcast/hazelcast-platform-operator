package integration

import (
	"context"
	"fmt"
	"net"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = FDescribe("HazelcastEndpoint CR", func() {
	const namespace = "default"

	getHazelcastService := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, timeout Duration) *corev1.Service {
		By("Listing the Hazelcast service")
		svc := &corev1.Service{}
		Eventually(func() *corev1.Service {
			err := k8sClient.Get(ctx, lookupKey(hz), svc)
			if kerrors.IsNotFound(err) {
				return nil
			}
			Expect(err).To(Not(HaveOccurred()))
			return svc
		}, timeout, interval).Should(Not(BeNil()))
		return svc
	}

	listHazelcastEndpoints := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, timeout Duration, expectedLen int) []*hazelcastv1alpha1.HazelcastEndpoint {
		By("Listing the HazelcastEndpoints")
		hzEndpointList := &hazelcastv1alpha1.HazelcastEndpointList{}
		labels := util.Labels(hz)
		nsOpt := client.InNamespace(hz.Namespace)
		lblOpt := client.MatchingLabels(labels)
		Eventually(func() []hazelcastv1alpha1.HazelcastEndpoint {
			err := k8sClient.List(ctx, hzEndpointList, nsOpt, lblOpt)
			Expect(err).To(Not(HaveOccurred()))
			return hzEndpointList.Items
		}, timeout, interval).Should(HaveLen(expectedLen))
		var hzEndpoints []*hazelcastv1alpha1.HazelcastEndpoint
		for _, hzep := range hzEndpointList.Items {
			hzEndpoints = append(hzEndpoints, hzep.DeepCopy())
		}
		return hzEndpoints
	}

	waitForHazelcastEndpointToHaveAddress := func(ctx context.Context, hzep *hazelcastv1alpha1.HazelcastEndpoint, timeout Duration) {
		By("Waiting for the HazelcastEndpoints to have an address")
		endpointLookup := lookupKey(hzep)
		Eventually(func() string {
			err := k8sClient.Get(ctx, endpointLookup, hzep)
			Expect(err).NotTo(HaveOccurred())
			return hzep.Status.Address
		}, timeout, interval).Should(Not(BeEmpty()))
	}

	waitForServiceToBeAssignedLoadBalancerAddress := func(ctx context.Context, svc *corev1.Service, timeout Duration) {
		By("Waiting for the Service to be assigned load balancer address")
		Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
		svcLookup := lookupKey(svc)
		Eventually(func() string {
			err := k8sClient.Get(ctx, svcLookup, svc)
			Expect(err).NotTo(HaveOccurred())
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				addr := util.GetLoadBalancerAddress(&ingress)
				if addr != "" {
					return addr
				}
			}
			return ""
		}, timeout, interval).Should(Not(BeEmpty()))
	}

	expectHazelcastEndpointMatchedWithService := func(hzep *hazelcastv1alpha1.HazelcastEndpoint, svc *corev1.Service) {
		By(fmt.Sprintf("Matching HazelcastEndpoints '%s' with Service '%s'", hzep.Name, svc.Name))
		Expect(hzep.Labels[n.HazelcastEndpointServiceLabelName]).Should(Equal(svc.Name))
		Expect(svc.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
		var addr string
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			addr = util.GetLoadBalancerAddress(&ingress)
			if addr != "" {
				break
			}
		}
		Expect(hzep.Status.Address).Should(Not(BeEmpty()))
		hzepHost, hzepPort, err := net.SplitHostPort(hzep.Status.Address)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(hzepHost).Should(Equal(addr))
		var ports []int32
		for _, svcPort := range svc.Spec.Ports {
			ports = append(ports, svcPort.Port)
		}
		hzPortInt, err := strconv.Atoi(hzepPort)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(ports).Should(ContainElement(int32(hzPortInt)))
	}

	getHazelcastMemberServices := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, timeout Duration) []corev1.Service {
		lbls := util.Labels(hz)
		lbls[n.ServicePerPodLabelName] = n.LabelValueTrue
		lblOpt := client.MatchingLabels(lbls)
		nsOpt := client.InNamespace(hz.Namespace)
		svcList := &corev1.ServiceList{}
		Eventually(func() []corev1.Service {
			err := k8sClient.List(ctx, svcList, lblOpt, nsOpt)
			Expect(err).ToNot(HaveOccurred())
			return svcList.Items
		}, timeout, interval).Should(HaveLen(int(*hz.Spec.ClusterSize)))
		return svcList.Items
	}

	getWanServices := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, timeout Duration) []*corev1.Service {
		By("Getting WAN services")
		var services []*corev1.Service
		for _, wan := range hz.Spec.AdvancedNetwork.WAN {
			svc := &corev1.Service{}
			svcName := fmt.Sprintf("%s-%s", hz.GetName(), wan.Name)
			Eventually(func() error {
				svcNn := types.NamespacedName{
					Namespace: hz.Namespace,
					Name:      svcName,
				}
				err := k8sClient.Get(ctx, svcNn, svc)
				return err
			}, timeout, interval).Should(Not(HaveOccurred()))
			services = append(services, svc)
		}
		return services
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

			getHazelcastService(ctx, hz, Minute)

			hzEndpoints := listHazelcastEndpoints(ctx, hz, Minute, 0)
			Expect(hzEndpoints).Should(BeEmpty())
		})
	})

	Context("Hazelcast that exposes external endpoints", func() {
		When("creating Hazelcast with expose externally enabled", func() {
			It("should create HazelcastEndpoint whose type is Discovery", Label("fast"), func() {
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
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
				}

				By("creating the Hazelcast CR with specs successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				svc := getHazelcastService(ctx, hz, Minute)
				hzEndpoints := listHazelcastEndpoints(ctx, hz, Minute, 1)

				waitForServiceToBeAssignedLoadBalancerAddress(ctx, svc, 20*Second)
				waitForHazelcastEndpointToHaveAddress(ctx, hzEndpoints[0], 20*Second)

				expectHazelcastEndpointMatchedWithService(hzEndpoints[0], svc)
				Expect(hzEndpoints[0].Spec.Type).Should(Equal(hazelcastv1alpha1.HazelcastEndpointTypeDiscovery))
			})

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
		})

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
	})
})
