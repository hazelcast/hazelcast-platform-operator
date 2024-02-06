package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

var _ = Describe("Hazelcast WAN Sync", Label("wan_sync"), func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.WanSync{}, &hazelcastcomv1alpha1.WanSyncList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, &hazelcastcomv1alpha1.WanReplicationList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Basic WAN Sync functionality", func() {
		It("should sync one map with another cluster", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("hws-1")

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSrcLookupKey.Name: {mapLookupKey.Name},
				hzTrgLookupKey.Name: nil,
			}, hzSrcLookupKey.Namespace, labels)
			mapSize := 1024
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPort, mapLookupKey.Name, mapSize)

			By("creating WAN configuration")
			wr := createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: mapLookupKey.Name},
				}, 1, labels)
			createWanSync(context.Background(), wanLookupKey, wr.Name, 1, labels)

			By("checking the size of the maps in the target cluster")
			waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPort, mapLookupKey.Name, mapSize, 1*Minute)
		})

		It("should sync two maps with another cluster", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			suffix := setLabelAndCRName("hws-2")

			// Hazelcast and Map CRs
			srcMap1 := "map1-" + suffix
			srcMap2 := "map2-" + suffix

			hzCrs, _ := createWanResources(context.Background(), map[string][]string{
				hzSrcLookupKey.Name: {srcMap1, srcMap2},
				hzTrgLookupKey.Name: nil,
			}, hzSrcLookupKey.Namespace, labels)
			mapSize := 1024
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPort, srcMap1, mapSize)
			fillTheMapDataPortForward(context.Background(), hzCrs[hzSrcLookupKey.Name], localPort, srcMap2, mapSize)

			By("creating WAN configuration")
			wr := createWanConfig(context.Background(), wanLookupKey, hzCrs[hzTrgLookupKey.Name],
				[]hazelcastcomv1alpha1.ResourceSpec{
					{Name: srcMap1},
					{Name: srcMap2},
				}, 2, labels)
			createWanSync(context.Background(), wanLookupKey, wr.Name, 2, labels)

			By("checking the size of the maps in the target cluster")
			waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPort, srcMap1, mapSize, 1*Minute)
			waitForMapSizePortForward(context.Background(), hzCrs[hzTrgLookupKey.Name], localPort, srcMap2, mapSize, 1*Minute)
		})
	})
})
