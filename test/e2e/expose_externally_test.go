package e2e

import (
	"context"
	"fmt"
	"net"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	hzCluster "github.com/hazelcast/hazelcast-go-client/cluster"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with expose externally feature", Group("expose_externally"), func() {
	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	ctx := context.Background()
	assertExternalAddressesNotEmpty := func() {
		By("status external addresses should not be empty")
		Eventually(func() []string {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(ctx, hzLookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			externalAddresses := fetchHazelcastAddressesByType(hz, hazelcastcomv1alpha1.HazelcastEndpointTypeDiscovery, hazelcastcomv1alpha1.HazelcastEndpointTypeMember)
			return externalAddresses
		}, 2*Minute, interval).Should(Not(BeEmpty()))
	}

	skipIfIstioSetup := func() {
		if isIstioSetup() {
			Skip("skipping suites in Istio setup")
		}
	}

	Context("Cluster connectivity", func() {
		It("should enable Hazelcast unisocket client connection to an externally exposed cluster", Tag(Kind), func() {
			skipIfIstioSetup()

			setLabelAndCRName("hee-1")
			hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			hzMap := "map"
			entryCount := 100
			err := FillMapByEntryCount(ctx, hzLookupKey, true, hzMap, entryCount)
			Expect(err).To(BeNil())
			WaitForMapSize(ctx, hzLookupKey, hzMap, entryCount, Minute)
			assertExternalAddressesNotEmpty()
		})

		It("should enable Hazelcast smart client connection to a cluster exposed with NodePort", Tag(AWS|GCP|AZURE), func() {
			skipIfIstioSetup()

			setLabelAndCRName("hee-2")
			hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			members := getHazelcastMembers(ctx, hazelcast)

			clientHz := GetHzClient(ctx, hzLookupKey, false)
			defer Expect(clientHz.Shutdown(ctx)).To(BeNil())
			internalClient := hzClient.NewClientInternal(clientHz)
			clientMembers := internalClient.OrderedMembers()
			Expect(members).Should(HaveLen(len(clientMembers)))

			By("matching HZ members with client members and comparing their public IPs")
			for _, member := range members {
				matched := false
				for _, clientMember := range clientMembers {
					if member.Uid != clientMember.UUID.String() {
						continue
					}
					matched = true

					service := getServiceOfMember(ctx, hzLookupKey.Namespace, member)
					Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
					Expect(service.Spec.Ports).Should(HaveLen(2))
					nodePort := service.Spec.Ports[0].NodePort
					node := getNodeOfMember(ctx, hzLookupKey.Namespace, member)
					externalAddresses := filterNodeAddressesByExternalIP(node.Status.Addresses)
					Expect(externalAddresses).Should(HaveLen(1))
					externalAddress := fmt.Sprintf("%s:%d", externalAddresses[0], nodePort)

					clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
					Expect(clientPublicAddresses).Should(HaveLen(1))
					clientPublicAddress := clientPublicAddresses[0]
					Expect(externalAddress).Should(Equal(clientPublicAddress))

					break
				}
				if !matched {
					Fail(fmt.Sprintf("member UID '%s' is not matched with client members UUIDs", member.Uid))
				}
			}

			By("checking if the client has connected to all the members")
			Eventually(func() bool {
				return clientConnectedToAllMembers(ctx, hzLookupKey)
			}, Minute, 10*Second).Should(BeTrue())

			assertExternalAddressesNotEmpty()

			Expect(FillMapByEntryCount(ctx, hzLookupKey, false, "map", 100)).To(BeNil())
			WaitForMapSize(ctx, hzLookupKey, "map", 100, Minute)
		})

		It("should enable Hazelcast smart client connection to a cluster exposed with LoadBalancer", Tag(AnyCloud), func() {
			skipIfIstioSetup()

			setLabelAndCRName("hee-3")
			hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzLookupKey, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			members := getHazelcastMembers(ctx, hazelcast)

			clientHz := GetHzClient(ctx, hzLookupKey, false)
			defer Expect(clientHz.Shutdown(ctx)).To(BeNil())
			internalClient := hzClient.NewClientInternal(clientHz)
			clientMembers := internalClient.OrderedMembers()
			Expect(members).Should(HaveLen(len(clientMembers)))

			By("matching HZ members with client members and comparing their public IPs")
			for _, member := range members {
				matched := false
				for _, clientMember := range clientMembers {
					if member.Uid != clientMember.UUID.String() {
						continue
					}
					matched = true

					service := getServiceOfMember(ctx, hzLookupKey.Namespace, member)
					Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
					Expect(service.Status.LoadBalancer.Ingress).Should(HaveLen(1))
					svcLoadBalancerIngress := service.Status.LoadBalancer.Ingress[0]

					clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
					Expect(clientPublicAddresses).Should(HaveLen(1))
					clientPublicIp, port, err := net.SplitHostPort(clientPublicAddresses[0])
					Expect(err).ToNot(HaveOccurred())
					Expect(port).Should(Equal("5701"))

					if svcLoadBalancerIngress.IP != "" {
						Expect(svcLoadBalancerIngress.IP).Should(Equal(clientPublicIp))
					} else if svcLoadBalancerIngress.Hostname != "" {
						Eventually(func() bool {
							matched, err := DnsLookupAddressMatched(ctx, svcLoadBalancerIngress.Hostname, clientPublicIp)
							if err != nil {
								return false
							}
							return matched
						}, 3*Minute, interval).Should(BeTrue())
					} else {
						Fail("expected LoadBalancer IP or Hostname to be non-empty")
					}

					break
				}
				if !matched {
					Fail(fmt.Sprintf("member UID '%s' is not matched with client members UUIDs", member.Uid))
				}
			}

			By("checking if the client has connected to all the members")
			Eventually(func() bool {
				return clientConnectedToAllMembers(ctx, hzLookupKey)
			}, Minute, 10*Second).Should(BeTrue())

			assertExternalAddressesNotEmpty()

			Expect(FillMapByEntryCount(ctx, hzLookupKey, false, "map", 100)).To(BeNil())
			WaitForMapSize(ctx, hzLookupKey, "map", 100, Minute)
		})
	})
})

func getHazelcastMembers(ctx context.Context, hazelcast *hazelcastcomv1alpha1.Hazelcast) []hazelcastcomv1alpha1.HazelcastMemberStatus {
	hz := &hazelcastcomv1alpha1.Hazelcast{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: hazelcast.Namespace, Name: hazelcast.Name}, hz)
	Expect(err).Should(BeNil())
	return hz.Status.Members
}

func getServiceOfMember(ctx context.Context, namespace string, member hazelcastcomv1alpha1.HazelcastMemberStatus) *corev1.Service {
	service := &corev1.Service{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: member.PodName}, service)
	Expect(err).Should(BeNil())
	return service
}

func getNodeOfMember(ctx context.Context, namespace string, member hazelcastcomv1alpha1.HazelcastMemberStatus) *corev1.Node {
	pod := &corev1.Pod{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: member.PodName}, pod)
	Expect(err).Should(BeNil())
	node := &corev1.Node{}
	err = k8sClient.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node)
	Expect(err).Should(BeNil())
	return node
}

func filterNodeAddressesByExternalIP(nodeAddresses []corev1.NodeAddress) []string {
	addresses := make([]string, 0)
	for _, addr := range nodeAddresses {
		if addr.Type == corev1.NodeExternalIP {
			addresses = append(addresses, addr.Address)
		}
	}
	return addresses
}

func filterClientMemberAddressesByPublicIdentifier(member hzCluster.MemberInfo) []string {
	addresses := make([]string, 0)
	for eq, addr := range member.AddressMap {
		if eq.Identifier == "public" {
			addresses = append(addresses, addr.String())
		}
	}
	return addresses
}

func clientConnectedToAllMembers(ctx context.Context, lk types.NamespacedName) bool {
	clientHz := GetHzClient(ctx, lk, false)
	defer Expect(clientHz.Shutdown(ctx)).To(BeNil())
	internalClient := hzClient.NewClientInternal(clientHz)
	clientMembers := internalClient.OrderedMembers()
	for _, member := range clientMembers {
		if !internalClient.ConnectedToMember(member.UUID) {
			return false
		}
	}
	return true
}

// checks if the istio system namespace exists and injection label is set in the test namespace
func isIstioSetup() bool {
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "istio-system"}, &corev1.Namespace{})
	if kerrors.IsNotFound(err) {
		return false
	}
	Expect(err).NotTo(HaveOccurred())

	namespace := &corev1.Namespace{}
	err = k8sClient.Get(context.Background(), client.ObjectKey{Name: hzNamespace}, namespace)
	Expect(err).NotTo(HaveOccurred())
	val, ok := namespace.Labels["istio-injection"]
	return ok && val == "enabled"
}
