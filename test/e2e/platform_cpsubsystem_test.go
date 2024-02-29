package e2e

import (
	"context"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("CP Subsystem", Label("cp_subsystem"), func() {
	//localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should store data into CP Map", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("cp-1")
		clusterSize := int32(3)
		ctx := context.Background()
		cpMapName := "my-map"

		hazelcast := hazelcastconfig.HazelcastCPSubsystem(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		clientHz := GetHzClient(ctx, hzLookupKey, true)
		cli := hzClient.NewClientInternal(clientHz)

		grResp, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPGroupCreateCPGroupRequest("new-group"), nil)
		Expect(err).To(BeNil())
		rg := codec.DecodeCPGroupCreateCPGroupResponse(grResp)

		key, _ := cli.EncodeData("key")
		value, _ := cli.EncodeData("value")
		_, err = cli.InvokeOnRandomTarget(ctx, codec.EncodeCPMapPutRequest(rg, cpMapName, key, value), nil)
		Expect(err).To(BeNil())

		r, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPMapGetRequest(rg, cpMapName, key), nil)
		response, err := cli.DecodeData(codec.DecodeCPMapGetResponse(r))
		Expect(err).To(BeNil())
		Expect(response).To(Equal("value"))
	})

	It("should restore the data from CP Persistence", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("cp-2")
		clusterSize := int32(3)
		ctx := context.Background()
		cpMapName := "my-map"

		hazelcast := hazelcastconfig.HazelcastCPSubsystem(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		clientHz := GetHzClient(ctx, hzLookupKey, true)
		cli := hzClient.NewClientInternal(clientHz)

		grResp, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPGroupCreateCPGroupRequest("new-group"), nil)
		Expect(err).To(BeNil())
		rg := codec.DecodeCPGroupCreateCPGroupResponse(grResp)

		key, _ := cli.EncodeData("key")
		value, _ := cli.EncodeData("value")
		_, err = cli.InvokeOnRandomTarget(ctx, codec.EncodeCPMapPutRequest(rg, cpMapName, key, value), nil)
		Expect(err).To(BeNil())

		By("pause Hazelcast")
		UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(0)
			return hazelcast
		})
		WaitForReplicaSize(hazelcast.Namespace, hazelcast.Name, 0)

		By("resume Hazelcast")
		UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(3)
			return hazelcast
		})
		evaluateReadyMembers(hzLookupKey)

		r, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPMapGetRequest(rg, cpMapName, key), nil)
		response, err := cli.DecodeData(codec.DecodeCPMapGetResponse(r))
		Expect(err).To(BeNil())
		Expect(response).To(Equal("value"))
	})
})
