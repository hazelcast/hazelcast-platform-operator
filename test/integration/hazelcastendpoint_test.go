package integration

import (
	"context"
	"fmt"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("HazelcastEndpoint CR", func() {
	const namespace = "default"

	listHazelcastEndpoints := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast) []hazelcastv1alpha1.HazelcastEndpoint {
		hzEndpointList := &hazelcastv1alpha1.HazelcastEndpointList{}
		labels := util.Labels(hz)
		nsOpt := client.InNamespace(hz.Namespace)
		lblOpt := client.MatchingLabels(labels)
		err := k8sClient.List(ctx, hzEndpointList, nsOpt, lblOpt)
		Expect(err).To(Not(HaveOccurred()))
		return hzEndpointList.Items
	}

	getHazelcastService := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast) *corev1.Service {
		svc := &corev1.Service{}
		err := k8sClient.Get(ctx, lookupKey(hz), svc)
		if kerrors.IsNotFound(err) {
			return nil
		}
		Expect(err).To(Not(HaveOccurred()))
		return svc
	}

	waitForHazelcastService := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, timeout Duration) *corev1.Service {
		By("Waiting for the Hazelcast service")
		var svc *corev1.Service
		Eventually(func() *corev1.Service {
			svc = getHazelcastService(ctx, hz)
			return svc
		}, timeout, interval).Should(Not(BeNil()))
		return svc
	}

	waitForHazelcastEndpoints := func(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast, timeout Duration, expectedLen int) []hazelcastv1alpha1.HazelcastEndpoint {
		By("Waiting for the HazelcastEndpoints")
		var hzEndpoints []hazelcastv1alpha1.HazelcastEndpoint
		Eventually(func() []hazelcastv1alpha1.HazelcastEndpoint {
			hzEndpoints = listHazelcastEndpoints(ctx, hz)
			return hzEndpoints
		}, timeout, interval).Should(HaveLen(expectedLen))
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
		//Expect(addr).Should(Equal(hzep.Status.Address))
		//Expect(svc.Spec.Ports)
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

			waitForHazelcastService(ctx, hz, Minute)

			hzEndpoints := listHazelcastEndpoints(ctx, hz)
			Expect(hzEndpoints).Should(BeEmpty())
		})
	})

	Context("Hazelcast which expose external endpoints", func() {
		When("creating Hazelcast with expose externally enabled", func() {
			It("should create HazelcastEndpoint whose type is Discovery", Label("fast"), func() {
				lbCtx, cancel := context.WithCancel(ctx)
				go AssignLoadBalancerAddress(lbCtx, k8sClient, 5*Second, client.InNamespace(namespace))
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

				svc := waitForHazelcastService(ctx, hz, Minute)
				hzEndpoints := waitForHazelcastEndpoints(ctx, hz, Minute, 1)

				waitForServiceToBeAssignedLoadBalancerAddress(ctx, svc, Minute)
				waitForHazelcastEndpointToHaveAddress(ctx, &hzEndpoints[0], Minute)

				expectHazelcastEndpointMatchedWithService(&hzEndpoints[0], svc)
				Expect(hzEndpoints)
			})
		})
	})
})
