package e2e

import (
	"context"
	"fmt"
	"strings"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	hzCluster "github.com/hazelcast/hazelcast-go-client/cluster"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
		evaluateReadyMembers(hzLookupKey, 3)

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
				nodePort := service.Spec.Ports[0].NodePort
				node := getNodeOfMember(ctx, hzLookupKey.Namespace, member)
				Expect(service.Spec.Ports).Should(HaveLen(1))
				externalAddresses := filterNodeAddressesByExternalIP(node.Status.Addresses)
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
		evaluateReadyMembers(hzLookupKey, 3)

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
				serviceExternalIP := getLoadBalancerIngressPublicIP(ctx, service.Status.LoadBalancer.Ingress[0])
				clientPublicAddresses := filterClientMemberAddressesByPublicIdentifier(clientMember)
				Expect(clientPublicAddresses).Should(HaveLen(1))
				clientPublicIp := clientPublicAddresses[0][:strings.IndexByte(clientPublicAddresses[0], ':')]
				Expect(serviceExternalIP).Should(Equal(clientPublicIp))
				continue memberLoop
			}
			Fail(fmt.Sprintf("member Uid '%s' is not matched with client members UUIDs", member.Uid))
		}

		FillTheMapData(ctx, hzLookupKey, false, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)

		assertExternalAddressesNotEmpty()
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

func getLoadBalancerIngressPublicIP(ctx context.Context, lbi corev1.LoadBalancerIngress) string {
	publicIP := lbi.IP
	if publicIP == "" {
		It("Lookup for hostname of load balancer ingress")
		Eventually(func() (err error) {
			publicIP, err = DnsLookup(ctx, lbi.Hostname)
			return
		}, 3*Minute, interval).Should(BeNil())
	}
	return publicIP
}
