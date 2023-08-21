package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast WAN", Label("hz_wan"), func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

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

	//When("All pods are deleted",
	It("should send data using WanReplication configuration in Config", Label("slow"), func() {
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
	})

	//When("Multiple WanReplication resources with multiple maps exist",
	It("should replicate maps to target cluster including maps with names different from map CR name", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		suffix := setLabelAndCRName("hw-3")

		// Hazelcast and Map CRs
		hzSource1 := "hz1-source" + suffix
		map11 := "map11" + suffix
		map12CrName := "map12" + suffix
		map12MapName := map12CrName + "-mapName"

		hzSource2 := "hz2-source" + suffix
		map21 := "map21" + suffix

		hzTarget1 := "hz1-target" + suffix

		hzCrs, _ := createWanResources(context.Background(), map[string][]string{
			hzSource1: {map11},
			hzSource2: {map21},
			hzTarget1: nil,
		}, hzNamespace, labels)
		// Create the map with CRName != mapName
		createMapCRWithMapName(context.Background(), map12CrName, map12MapName, types.NamespacedName{Name: hzSource1, Namespace: hzNamespace})

		By("creating first WAN configuration")
		createWanConfig(context.Background(), types.NamespacedName{Name: "wan1" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
			[]hazelcastcomv1alpha1.ResourceSpec{
				{Name: map12CrName, Kind: hazelcastcomv1alpha1.ResourceKindMap},
				{Name: hzSource2, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
			}, 2, labels)

		By("creating second WAN configuration")
		createWanConfig(context.Background(), types.NamespacedName{Name: "wan2" + suffix, Namespace: hzNamespace}, hzCrs[hzTarget1],
			[]hazelcastcomv1alpha1.ResourceSpec{
				{Name: map11},
			}, 1, labels)

		By("filling the maps in the source clusters")
		mapSize := 10
		fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map11, mapSize)
		fillTheMapDataPortForward(context.Background(), hzCrs[hzSource1], localPort, map12MapName, mapSize)
		fillTheMapDataPortForward(context.Background(), hzCrs[hzSource2], localPort, map21, mapSize)

		By("checking the size of the maps in the target cluster")
		waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map11, mapSize, 1*Minute)
		waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map12MapName, mapSize, 1*Minute)
		waitForMapSizePortForward(context.Background(), hzCrs[hzTarget1], localPort, map21, mapSize, 1*Minute)
	})

	//When("WAN replicated Map CR is deleted which was given as a Map resource in WAN spec",
	It("should delete the map from status and WAN status should be failed", Label("slow"), func() {
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
	})

	//When("WAN replicated Map CR is deleted which was given as a Hazelcast resource in WAN spec",
	It("should delete the map from status and WAN status should be Pending", Label("slow"), func() {
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
	})

	//When("WAN replicated Hazelcast CR is first deleted and then removed from the WAN spec",
	It("should fail first and after spec removal, should succeed", Label("slow"), func() {
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
		wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusFailed)

		wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
			{Name: hzSource1, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
		}
		Expect(k8sClient.Update(context.Background(), wan)).Should(BeNil())
		wan = assertWanStatusMapCount(wan, 3)
		_ = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
	})

	//When("WAN replicated maps are removed from WAN spec",
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
	})

	//When("Map is given twice in the WAN spec",
	It("should continue replication if one reference is deleted", Label("slow"), func() {
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
	})

	It("should start WanReplication for the maps created after the Wan CR", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		suffix := setLabelAndCRName("hw-9")

		// Hazelcast CRs
		hzSource := "hz-source" + suffix
		hzTarget := "hz-target" + suffix

		hzSourceCr := hazelcastconfig.Default(types.NamespacedName{
			Name:      hzSource,
			Namespace: hzSrcLookupKey.Namespace,
		}, ee, labels)
		hzSourceCr.Spec.ClusterName = hzSource
		hzSourceCr.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCRWithoutCheck(hzSourceCr)
		evaluateReadyMembers(types.NamespacedName{Name: hzSource, Namespace: hzSrcLookupKey.Namespace})

		hzTargetCr := hazelcastconfig.Default(types.NamespacedName{
			Name:      hzTarget,
			Namespace: hzSrcLookupKey.Namespace,
		}, ee, labels)
		hzTargetCr.Spec.ClusterName = hzTarget
		hzTargetCr.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCRWithoutCheck(hzTargetCr)
		evaluateReadyMembers(types.NamespacedName{Name: hzTarget, Namespace: hzTrgLookupKey.Namespace})

		// Map CRs
		mapBeforeWan := "map0" + suffix
		mapAfterWan := "map1" + suffix

		// Creating the map before the WanReplication CR
		mapBeforeWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapBeforeWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
		Expect(k8sClient.Create(context.Background(), mapBeforeWanCr)).Should(Succeed())
		assertMapStatus(mapBeforeWanCr, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration")
		wan := hazelcastconfig.WanReplication(
			wanLookupKey,
			hzTargetCr.Spec.ClusterName,
			fmt.Sprintf("%s.%s.svc.cluster.local:%d", hzTargetCr.Name, hzTargetCr.Namespace, n.WanDefaultPort),
			[]hazelcastcomv1alpha1.ResourceSpec{
				{
					Name: hzSource,
					Kind: hazelcastcomv1alpha1.ResourceKindHZ,
				},
			},
			labels,
		)
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
		wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

		assertWanStatusMapCount(wan, 1)

		mapSize := 1024

		By("checking the size of the map created before the wan")
		fillTheMapDataPortForward(context.Background(), hzSourceCr, localPort, mapBeforeWan, mapSize)
		waitForMapSizePortForward(context.Background(), hzTargetCr, localPort, mapBeforeWan, mapSize, Minute)

		// Creating the map after the WanReplication CR
		mapAfterWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapAfterWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
		Expect(k8sClient.Create(context.Background(), mapAfterWanCr)).Should(Succeed())
		assertMapStatus(mapAfterWanCr, hazelcastcomv1alpha1.MapSuccess)

		assertWanStatusMapCount(wan, 2)

		By("checking the size of the map created after the wan")
		fillTheMapDataPortForward(context.Background(), hzSourceCr, localPort, mapAfterWan, mapSize)
		waitForMapSizePortForward(context.Background(), hzTargetCr, localPort, mapAfterWan, mapSize, Minute)
	})

	It("should set WanReplication CR status to Success when resource map is created after the wan", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		suffix := setLabelAndCRName("hw-10")

		// Hazelcast CRs
		hzSource := "hz-source" + suffix
		hzTarget := "hz-target" + suffix

		hzSourceCr := hazelcastconfig.Default(types.NamespacedName{
			Name:      hzSource,
			Namespace: hzSrcLookupKey.Namespace,
		}, ee, labels)
		hzSourceCr.Spec.ClusterName = hzSource
		hzSourceCr.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCRWithoutCheck(hzSourceCr)
		evaluateReadyMembers(types.NamespacedName{Name: hzSource, Namespace: hzSrcLookupKey.Namespace})

		hzTargetCr := hazelcastconfig.Default(types.NamespacedName{
			Name:      hzTarget,
			Namespace: hzSrcLookupKey.Namespace,
		}, ee, labels)
		hzTargetCr.Spec.ClusterName = hzTarget
		hzTargetCr.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCRWithoutCheck(hzTargetCr)
		evaluateReadyMembers(types.NamespacedName{Name: hzTarget, Namespace: hzTrgLookupKey.Namespace})

		// Map CRs
		mapBeforeWan := "map0" + suffix
		mapAfterWan := "map1" + suffix

		// Creating the map before the WanReplication CR
		mapBeforeWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapBeforeWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
		Expect(k8sClient.Create(context.Background(), mapBeforeWanCr)).Should(Succeed())
		assertMapStatus(mapBeforeWanCr, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration")
		wan := hazelcastconfig.WanReplication(
			wanLookupKey,
			hzTargetCr.Spec.ClusterName,
			hzclient.HazelcastUrl(hzTargetCr),
			[]hazelcastcomv1alpha1.ResourceSpec{
				{
					Name: mapBeforeWan,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				},
				{
					Name: mapAfterWan,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				},
			},
			labels,
		)
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
		wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusFailed)

		// Creating the map after the WanReplication CR
		mapAfetWanCr := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapAfterWan, Namespace: mapLookupKey.Namespace}, hzSource, labels)
		Expect(k8sClient.Create(context.Background(), mapAfetWanCr)).Should(Succeed())
		assertMapStatus(mapAfetWanCr, hazelcastcomv1alpha1.MapSuccess)

		wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
		assertWanStatusMapCount(wan, 2)
	})

})
