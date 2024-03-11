package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Tiered Storage feature enabled", Group("tiered_storage"), func() {
	localPort := strconv.Itoa(8300 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1alpha1.Map{}, &hazelcastv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())

	})
	Context("Tiered Store enabled for map", func() {
		It("should create Tiered Store Configs with correct default values", Tag(EE|Kind|AnyCloud), func() {
			setLabelAndCRName("hts-1")

			By("creating the Hazelcast with Local Device config")
			deviceName := "test-device"
			hazelcast := hazelcastconfig.HazelcastTieredStorage(hzLookupKey, deviceName, labels)
			hazelcast.Spec.Properties = map[string]string{"hazelcast.hidensity.check.freememory": "false"}
			hazelcast.Spec.NativeMemory.Size = []resource.Quantity{resource.MustParse("1200M")}[0]
			CreateHazelcastCR(hazelcast)

			By("creating the map config with Tiered Store config")
			tsm := hazelcastconfig.DefaultTieredStoreMap(mapLookupKey, hazelcast.Name, deviceName, labels)
			Expect(k8sClient.Create(context.Background(), tsm)).Should(Succeed())
			assertMapStatus(tsm, hazelcastv1alpha1.MapSuccess)

			By("checking if the TS map config is created correctly")
			memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
			mapConfig := getMapConfigFromMemberConfig(memberConfigXML, tsm.MapName())

			Expect(mapConfig).NotTo(BeNil())
			Expect(mapConfig.TieredStoreConfig.Enabled).Should(Equal(true))
			Expect(mapConfig.TieredStoreConfig.DiskTierConfig.Enabled).Should(Equal(true))
			Expect(mapConfig.TieredStoreConfig.DiskTierConfig.DeviceName).Should(Equal(deviceName))
			Expect(mapConfig.TieredStoreConfig.MemoryTierConfig.Capacity.Unit).Should(Equal("BYTES"))
			Expect(mapConfig.TieredStoreConfig.MemoryTierConfig.Capacity.Value).Should(Equal(int64(256000000)))
			Expect(mapConfig.InMemoryFormat).Should(Equal(string(tsm.Spec.InMemoryFormat)))
		})

	})

})
