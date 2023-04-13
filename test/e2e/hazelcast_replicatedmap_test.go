package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	hazelcastcomv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast ReplicatedMap Config", Label("replicatedmap"), func() {
	localPort := strconv.Itoa(8600 + GinkgoParallelProcess())

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
		DeleteAllOf(&hazelcastcomv1beta1.ReplicatedMap{}, &hazelcastcomv1beta1.ReplicatedMapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1beta1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1beta1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create ReplicatedMap Config", Label("fast"), func() {
		setLabelAndCRName("hrm-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		rm := hazelcastconfig.DefaultReplicatedMap(rmLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
		assertDataStructureStatus(rmLookupKey, hazelcastcomv1beta1.DataStructureSuccess, &hazelcastcomv1beta1.ReplicatedMap{})
	})

	It("should create ReplicatedMap Config with correct default values", Label("fast"), func() {
		setLabelAndCRName("hrm-2")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the default ReplicatedMap config")
		rm := hazelcastconfig.DefaultReplicatedMap(rmLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
		rm = assertDataStructureStatus(rmLookupKey, hazelcastcomv1beta1.DataStructureSuccess, &hazelcastcomv1beta1.ReplicatedMap{}).(*hazelcastcomv1beta1.ReplicatedMap)

		memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
		replicatedMapConfig := getReplicatedMapConfigFromMemberConfig(memberConfigXML, rm.GetDSName())
		Expect(replicatedMapConfig).NotTo(BeNil())

		Expect(replicatedMapConfig.InMemoryFormat).Should(Equal(n.DefaultReplicatedMapInMemoryFormat))
		Expect(replicatedMapConfig.AsyncFillup).Should(Equal(n.DefaultReplicatedMapAsyncFillup))
	})

	It("should fail to update ReplicatedMap Config", Label("fast"), func() {
		setLabelAndCRName("hrm-3")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the ReplicatedMap config")
		rms := hazelcastcomv1beta1.ReplicatedMapSpec{
			HazelcastResourceName: hzLookupKey.Name,
			InMemoryFormat:        hazelcastcomv1beta1.RMInMemoryFormatBinary,
			AsyncFillup:           pointer.Bool(false),
		}
		rm := hazelcastconfig.ReplicatedMap(rms, rmLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
		rm = assertDataStructureStatus(rmLookupKey, hazelcastcomv1beta1.DataStructureSuccess, &hazelcastcomv1beta1.ReplicatedMap{}).(*hazelcastcomv1beta1.ReplicatedMap)

		By("failing to update ReplicatedMap config")
		rm.Spec.InMemoryFormat = hazelcastcomv1beta1.RMInMemoryFormatObject
		rm.Spec.AsyncFillup = pointer.Bool(true)
		Expect(k8sClient.Update(context.Background(), rm)).Should(Succeed())
		assertDataStructureStatus(rmLookupKey, hazelcastcomv1beta1.DataStructureFailed, &hazelcastcomv1beta1.ReplicatedMap{})
	})
})
