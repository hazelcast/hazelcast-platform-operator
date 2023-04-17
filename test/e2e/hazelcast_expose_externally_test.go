package e2e

import (
	"context"
	"fmt"
	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"strings"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	hzCluster "github.com/hazelcast/hazelcast-go-client/cluster"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with expose externally feature", Label("hz_expose_externally"), func() {

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
	})

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
		Eventually(func() string {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(ctx, hzLookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.ExternalAddresses
		}, 2*Minute, interval).Should(Not(BeEmpty()))
	}

	It("should create Hazelcast cluster and allow connecting with Hazelcast unisocket client", Label("slow"), func() {
		setLabelAndCRName("hee-1")
		hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		FillTheMapData(ctx, hzLookupKey, true, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)

		assertExternalAddressesNotEmpty()
	})

	It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", Label("slow"), func() {
		setLabelAndCRName("hee-2")
		hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		members := getHazelcastMembers(ctx, hazelcast)
		clientHz := GetHzClient(ctx, hzLookupKey, false)
		defer Expect(clientHz.Shutdown(ctx)).To(BeNil())
		clientMembers := hzClient.NewClientInternal(clientHz).OrderedMembers()

		By("matching HZ members with client members and comparing their public IPs")

	memberLoop:
		for _, member := range members {
			for _, clientMember := range clientMembers {
				if member.Uid != clientMember.UUID.String() {
					continue
				}
				service := getServiceOfMember(ctx, hzLookupKey.Namespace, member)
				Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
				Expect(service.Spec.Ports).Should(HaveLen(1))
				nodePort := service.Spec.Ports[0].NodePort
				node := getNodeOfMember(ctx, hzLookupKey.Namespace, member)
				externalAddresses := filterNodeAddressesByExternalIP(node.Status.Addresses)
				// skip member IP check if the node has no external IP
				if len(externalAddresses) == 0 {
					continue memberLoop
				}
				Expect(externalAddresses).Should(HaveLen(1))
				externalAddress := fmt.Sprintf("%s:%d", externalAddresses[0], nodePort)
				clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
				Expect(clientPublicAddresses).Should(HaveLen(1))
				clientPublicAddress := clientPublicAddresses[0]
				Expect(externalAddress).Should(Equal(clientPublicAddress))
				continue memberLoop
			}
			Fail(fmt.Sprintf("member Uid '%s' is not matched with client members UUIDs", member.Uid))
		}

		FillTheMapData(ctx, hzLookupKey, false, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)

		assertExternalAddressesNotEmpty()
	})

	It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", Label("slow"), func() {
		setLabelAndCRName("hee-3")
		hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		members := getHazelcastMembers(ctx, hazelcast)
		clientHz := GetHzClient(ctx, hzLookupKey, false)
		defer Expect(clientHz.Shutdown(ctx)).To(BeNil())
		clientMembers := hzClient.NewClientInternal(clientHz).OrderedMembers()

		By("matching HZ members with client members and comparing their public IPs")

	memberLoop:
		for _, member := range members {
			for _, clientMember := range clientMembers {
				if member.Uid != clientMember.UUID.String() {
					continue
				}
				service := getServiceOfMember(ctx, hzLookupKey.Namespace, member)
				Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
				Expect(service.Status.LoadBalancer.Ingress).Should(HaveLen(1))
				svcLoadBalancerIngress := service.Status.LoadBalancer.Ingress[0]
				clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
				Expect(clientPublicAddresses).Should(HaveLen(1))
				clientPublicIp := clientPublicAddresses[0][:strings.IndexByte(clientPublicAddresses[0], ':')]
				if svcLoadBalancerIngress.IP != "" {
					Expect(svcLoadBalancerIngress.IP).Should(Equal(clientPublicIp))
				} else {
					hostname := svcLoadBalancerIngress.Hostname
					Eventually(func() bool {
						matched, err := DnsLookupAddressMatched(ctx, hostname, clientPublicIp)
						if err != nil {
							return false
						}
						return matched
					}, 3*Minute, interval).Should(BeTrue())

				}
				continue memberLoop
			}
			Fail(fmt.Sprintf("member Uid '%s' is not matched with client members UUIDs", member.Uid))
		}

		FillTheMapData(ctx, hzLookupKey, false, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)

		assertExternalAddressesNotEmpty()
	})
})

func getHazelcastMembers(ctx context.Context, hazelcast *hazelcastv1beta1.Hazelcast) []hazelcastv1beta1.HazelcastMemberStatus {
	hz := &hazelcastv1beta1.Hazelcast{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: hazelcast.Namespace, Name: hazelcast.Name}, hz)
	Expect(err).Should(BeNil())
	return hz.Status.Members
}

func getServiceOfMember(ctx context.Context, namespace string, member hazelcastv1beta1.HazelcastMemberStatus) *corev1.Service {
	service := &corev1.Service{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: member.PodName}, service)
	Expect(err).Should(BeNil())
	return service
}

func getNodeOfMember(ctx context.Context, namespace string, member hazelcastv1beta1.HazelcastMemberStatus) *corev1.Node {
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
