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
		fmt.Printf("members len: %v\n", len(members))
		fmt.Printf("members: %v\n", members)
		clientMembers := GetHzClientMembers(ctx, hzLookupKey, false)
		fmt.Printf("clientMembers: %v\n", clientMembers)

	memberLoop:
		for _, member := range members {
			for _, clientMember := range clientMembers {
				if member.Uid == clientMember.UUID.String() {
					fmt.Printf("matched member: %v\n", member)
					//Expect(member.Ip).Should(Equal(clientMember.Address.String()))
					//Expect(clientMember.AddressMap).Should(HaveLen(2))
					service := getServiceOfMember(ctx, member)
					node := getNodeOfMember(ctx, member)
					fmt.Printf("service: %v", service)
					fmt.Printf("node: %v", node)
					endpoints := combineNodeAddressesWithServicePods(node.Status.Addresses, service.Spec.Ports)
					fmt.Printf("endpoints: %v\n", endpoints)
					Expect(endpoints).Should(HaveLen(1))
					for endpointQualifier, cliMemberAddress := range clientMember.AddressMap {
						if endpointQualifier.Identifier == "public" {
							Expect(cliMemberAddress.String()).Should(Equal(endpoints[0]))
						} else {
							Expect(cliMemberAddress.String()).Should(Equal(member.Ip))
						}
					}
					continue memberLoop
				}
			}
			Fail("member UUID and client member UUID is not matched")
		}
		FillTheMapData(ctx, hzLookupKey, false, "map", 100)
		WaitForMapSize(ctx, hzLookupKey, "map", 100, 1*Minute)
	})

	FIt("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", Label("slow"), func() {
		setLabelAndCRName("hee-3")
		hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)
		members := getHazelcastMembers(ctx, hazelcast)
		clientMembers := GetHzClientMembers(ctx, hzLookupKey, false)
	memberLoop:
		for _, member := range members {
			for _, clientMember := range clientMembers {
				if member.Uid == clientMember.UUID.String() {
					Expect(member.Ip).Should(Equal(ipOfClientMemberAddress(clientMember.Address)))
					service := getServiceOfMember(ctx, member)
					Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeLoadBalancer))
					Expect(service.Status.LoadBalancer.Ingress).Should(HaveLen(1))
					Expect(clientMember.AddressMap).Should(HaveLen(2))
					for endpointQualifier, cliMemberAddress := range clientMember.AddressMap {
						if endpointQualifier.Identifier == "public" {
							Expect(ipOfClientMemberAddress(cliMemberAddress)).Should(Equal(service.Status.LoadBalancer.Ingress[0].IP))
						} else {
							Expect(ipOfClientMemberAddress(cliMemberAddress)).Should(Equal(member.Ip))
						}
					}
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
	fmt.Printf("getting service of member, name: %s\n", member.PodName)
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: member.PodName}, service)
	Expect(err).Should(BeNil())
	return service
}

func getNodeOfMember(ctx context.Context, member hazelcastcomv1alpha1.HazelcastMemberStatus) *corev1.Node {
	pod := &corev1.Pod{}
	fmt.Printf("getting pod of member, name: %s\n", member.PodName)
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: member.PodName}, pod)
	Expect(err).Should(BeNil())
	node := &corev1.Node{}
	fmt.Printf("getting pod of pod, name: %s\n", member.PodName)
	err = k8sClient.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node)
	Expect(err).Should(BeNil())
	return node
}

func combineNodeAddressesWithServicePods(nodeAddresses []corev1.NodeAddress, servicePorts []corev1.ServicePort) []string {
	endpoints := make([]string, len(nodeAddresses))
	for _, nodeAddress := range nodeAddresses {
		for _, servicePort := range servicePorts {
			endpoints = append(endpoints, fmt.Sprintf("%s:%d", nodeAddress.Address, servicePort.NodePort))
		}
	}
	return endpoints
}

func ipOfClientMemberAddress(addr hzCluster.Address) string {
	return string(addr[:strings.IndexByte(string(addr), ':')])
}
