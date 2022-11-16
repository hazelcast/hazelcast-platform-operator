package e2e

import (
	"context"
	"fmt"
	hzCluster "github.com/hazelcast/hazelcast-go-client/cluster"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

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
		By("checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
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
		//todo: test env nodes has public ip address
		//clientInternal := GetHzClientMembers(ctx, hzLookupKey, true)
		FillTheMapData(ctx, hzLookupKey, true, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)
		assertExternalAddressesNotEmpty()
	})

	It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", Label("slow"), func() {
		setLabelAndCRName("hee-2")
		hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		members := getHazelcastMembers(ctx, hazelcast)
		clientMembers := GetHzClientMembers(ctx, hzLookupKey, false)

		By("matching HZ members with client members and comparing their public IPs")

	memberLoop:
		for _, member := range members {
			for _, clientMember := range clientMembers {
				if member.Uid == clientMember.UUID.String() {
					service := getServiceOfMember(ctx, member)
					Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
					node := getNodeOfMember(ctx, member)
					Expect(service.Spec.Ports).Should(HaveLen(1))
					nodePort := service.Spec.Ports[0].NodePort
					externalAddresses := filterNodeAddressesByExternalIP(node.Status.Addresses)
					Expect(externalAddresses).Should(HaveLen(1))
					externalAddress := fmt.Sprintf("%s:%d", externalAddresses[0], nodePort)
					clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
					Expect(clientPublicAddresses).Should(HaveLen(1))
					clientPublicAddress := clientPublicAddresses[0]
					Expect(externalAddress).Should(Equal(clientPublicAddress))
					continue memberLoop
				}
			}
			Fail(fmt.Sprintf("member Uid '%s' is not matched with client members UUIDs", member.Uid))
		}

		FillTheMapData(ctx, hzLookupKey, false, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)
	})

	It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", Label("slow"), func() {
		setLabelAndCRName("hee-3")
		hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		members := getHazelcastMembers(ctx, hazelcast)
		clientMembers := GetHzClientMembers(ctx, hzLookupKey, false)

		By("matching HZ members with client members and comparing their public IPs")

	memberLoop:
		for _, member := range members {
			for _, clientMember := range clientMembers {
				if member.Uid == clientMember.UUID.String() {
					service := getServiceOfMember(ctx, member)
					Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
					Expect(service.Status.LoadBalancer.Ingress).Should(HaveLen(1))
					serviceExternalIP := service.Status.LoadBalancer.Ingress[0].IP
					clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
					Expect(clientPublicAddresses).Should(HaveLen(1))
					clientPublicIp := clientPublicAddresses[0][:strings.IndexByte(clientPublicAddresses[0], ':')]
					Expect(serviceExternalIP).Should(Equal(clientPublicIp))
					continue memberLoop
				}
			}
			Fail(fmt.Sprintf("member Uid '%s' is not matched with client members UUIDs", member.Uid))
		}

		FillTheMapData(ctx, hzLookupKey, false, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)
	})
})

func getHazelcastMembers(ctx context.Context, hazelcast *hazelcastcomv1alpha1.Hazelcast) []hazelcastcomv1alpha1.HazelcastMemberStatus {
	hz := &hazelcastcomv1alpha1.Hazelcast{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: hazelcast.Namespace, Name: hazelcast.Name}, hz)
	Expect(err).Should(BeNil())
	return hz.Status.Members
}

func getServiceOfMember(ctx context.Context, member hazelcastcomv1alpha1.HazelcastMemberStatus) *corev1.Service {
	service := &corev1.Service{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: member.PodName}, service)
	Expect(err).Should(BeNil())
	return service
}

func getNodeOfMember(ctx context.Context, member hazelcastcomv1alpha1.HazelcastMemberStatus) *corev1.Node {
	pod := &corev1.Pod{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: member.PodName}, pod)
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
