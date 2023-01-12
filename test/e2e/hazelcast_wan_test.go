package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

var _ = Describe("Hazelcast WAN", Label("hz_wan"), func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

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
		DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, &hazelcastcomv1alpha1.WanReplicationList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should send data to another cluster", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hw-1")

		hzCrs, _ := createWanResources(context.Background(), map[string][]string{
			hzSrcLookupKey.Name: {mapLookupKey.Name},
			hzTrgLookupKey.Name: nil,
		}, hzSrcLookupKey.Namespace, labels)

		By("creating WAN configuration")
		createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
			[]hazelcastcomv1alpha1.ResourceSpec{
				{Name: mapLookupKey.Name},
			}, 1, labels)

		By("checking the size of the maps in the target cluster")
		mapSize := 1024
		fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPort, mapLookupKey.Name, mapSize)
		waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPort, mapLookupKey.Name, mapSize, 1*Minute)
	})

	When("All pods are deleted",
		It("should send data to another cluster using ConfigMap configuration", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("hw-2")

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSrcLookupKey.Name: {mapLookupKey.Name},
				hzTrgLookupKey.Name: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating WAN configuration")
			createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: mapLookupKey.Name},
				}, 1, labels)

			deletePods(hzSrcLookupKey)
			// Wait for pod to be deleted
			Sleep(5 * Second)
			evaluateReadyMembers(hzSrcLookupKey)

			By("checking the size of the maps in the target cluster")
			mapSize := 1024
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPort, mapLookupKey.Name, mapSize)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPort, mapLookupKey.Name, mapSize, 1*Minute)
		}))

	When("Multiple WanReplication resources with multiple maps exist",
		It("should replicate maps to target cluster ", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hw-3")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix

			hzSource2 := "hz2-source" + suffix
			map21 := "map21" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12},
				hzSource2: {map21},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map11, Kind: hazelcastcomv1alpha1.ResourceKindMap},
					{Name: hzSource2, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 2, labels)

			By("creating second WAN configuration")
			createWanConfig(context.Background(), types.NamespacedName{Name: "wan2" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map12},
				}, 1, labels)

			By("filling the maps in the source clusters")
			mapSize := 10
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map11, mapSize)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map12, mapSize)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPort, map21, mapSize)

			By("checking the size of the maps in the target cluster")
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map11, mapSize, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map21, mapSize, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map12, mapSize, 1*Minute)
		}))

	When("Wan replicated Map CR is deleted which was given as a Map resource in Wan spec",
		It("should delete the map from status and Wan status should be failed", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hw-4")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, mapCrs := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map11, Kind: hazelcastcomv1alpha1.ResourceKindMap},
				}, 1, labels)

			Expect(k8sClient.Delete(context.Background(), mapCrs[map11])).Should(BeNil())
			assertObjectDoesNotExist(mapCrs[map11])
			wan = assertWanStatusMapCount(wan, 0)
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusFailed)

			Expect(k8sClient.Delete(context.Background(), wan)).Should(BeNil())
			assertObjectDoesNotExist(wan)
		}))

	When("Wan replicated Map CR is deleted which was given as a Hazelcast resource in Wan spec",
		It("should delete the map from status and Wan status should be Pending", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hw-5")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, mapCrs := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 1, labels)

			Expect(k8sClient.Delete(context.Background(), mapCrs[map11])).Should(BeNil())
			assertObjectDoesNotExist(mapCrs[map11])
			wan = assertWanStatusMapCount(wan, 0)
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusPending)

			Expect(k8sClient.Delete(context.Background(), wan)).Should(BeNil())
			assertObjectDoesNotExist(wan)
		}))

	When("Wan replicated Hazelcast CR is deleted which was given as a Hazelcast resource in Wan spec",
		It("should delete the maps from status and Wan status should be Pending ", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hw-6")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix
			map13 := "map13" + suffix

			hzSource2 := "hz2-source" + suffix
			map21 := "map21" + suffix
			map22 := "map22" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12, map13},
				hzSource2: {map21, map22},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
					{Name: hzSource2, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 5, labels)

			Expect(k8sClient.Delete(context.Background(), hzCrs[hzSource2])).Should(BeNil())
			assertObjectDoesNotExist(hzCrs[hzSource2])
			wan = assertWanStatusMapCount(wan, 3)
			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

			Expect(k8sClient.Delete(context.Background(), hzCrs[hzSource1])).Should(BeNil())
			assertObjectDoesNotExist(hzCrs[hzSource1])
			wan = assertWanStatusMapCount(wan, 0)
			_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusPending)
		}))

	When("Wan replicated maps are removed from Wan spec",
		It("should stop replicating to target cluster", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hw-7")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix

			hzSource2 := "hz2-source" + suffix
			map21 := "map21" + suffix
			map22 := "map22" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12},
				hzSource2: {map21, map22},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
					{Name: map21, Kind: hazelcastcomv1alpha1.ResourceKindMap},
					{Name: map22, Kind: hazelcastcomv1alpha1.ResourceKindMap},
				}, 4, labels)

			By("filling the maps in the source clusters")
			entryCount := 10
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map11, entryCount)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map12, entryCount)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPort, map21, entryCount)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPort, map22, entryCount)

			By("checking the size of the maps in the target cluster")
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map11, entryCount, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map12, entryCount, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map21, entryCount, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map22, entryCount, 1*Minute)

			By("stopping replicating all maps but map22")

			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
			wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
				{Name: map22, Kind: hazelcastcomv1alpha1.ResourceKindMap},
			}
			Expect(k8sClient.Update(context.Background(), wan)).Should(BeNil())
			wan = assertWanStatusMapCount(wan, 1)
			_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

			currentSize := entryCount
			newEntryCount := 50
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPort, map22, newEntryCount)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map11, newEntryCount)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map12, newEntryCount)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPort, map21, newEntryCount)

			By("checking the size of the maps in the target cluster")
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map22, currentSize+newEntryCount, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map11, currentSize, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map12, currentSize, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map21, currentSize, 1*Minute)
		}))

	When("Map is given twice in the Wan spec",
		It("should continue replication if one reference is deleted ", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hw-8")

			// Hazelcast and Map CRs
			hzSource1 := "hz1-source" + suffix
			map11 := "map11" + suffix
			map12 := "map12" + suffix

			hzTarget1 := "hz1-target" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSource1: {map11, map12},
				hzTarget1: nil,
			}, hzSrcLookupKey.Namespace, labels)

			By("creating first WAN configuration")
			wan := createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: map11, Kind: hazelcastcomv1alpha1.ResourceKindMap},
					{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
				}, 2, labels)

			wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
			wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
				{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
			}
			Expect(k8sClient.Update(context.Background(), wan)).Should(BeNil())
			Sleep(5 * Second)
			wan = assertWanStatusMapCount(wan, 2)
			_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
		}))
})
