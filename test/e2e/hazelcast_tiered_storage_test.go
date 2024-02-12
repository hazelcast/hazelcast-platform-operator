package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Tiered Storage feature enabled", Label("tiered_storage"), func() {
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

	It("should create Tiered Store Configs with correct default values", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hts-1")

		By("creating the Hazelcast with Local Device config")
		deviceName := "test-device"
		hazelcast := hazelcastconfig.HazelcastTieredStorage(hzLookupKey, deviceName, labels)
		hazelcast.Spec.NativeMemory.Size = []resource.Quantity{resource.MustParse("512M")}[0]
		CreateHazelcastCR(hazelcast)

		By("creating the map config with Tiered Store config")
		tsm := hazelcastconfig.DefaultTieredStoreMap(mapLookupKey, hazelcast.Name, deviceName, labels)
		Expect(k8sClient.Create(context.Background(), tsm)).Should(Succeed())
		assertMapStatus(tsm, hazelcastv1alpha1.MapSuccess)

		By("checking if the TS map config is created correctly")
		mapConfig := mapConfigPortForward(context.Background(), hazelcast, localPort, tsm.MapName())

		Expect(mapConfig).NotTo(BeNil())
		Expect(mapConfig.TieredStoreConfig.Enabled).Should(Equal(true))
		Expect(mapConfig.TieredStoreConfig.DiskTierConfig.Enabled).Should(Equal(true))
		Expect(mapConfig.TieredStoreConfig.DiskTierConfig.DeviceName).Should(Equal(deviceName))
		Expect(mapConfig.TieredStoreConfig.MemoryTierConfig.Capacity.Unit).Should(Equal(0))
		Expect(mapConfig.TieredStoreConfig.MemoryTierConfig.Capacity.Value).Should(Equal(256000000))
		Expect(mapConfig.InMemoryFormat).Should(Equal(hazelcastv1alpha1.EncodeInMemoryFormat[tsm.Spec.InMemoryFormat]))
	})

	It("should successfully fill the map with more than allocated memory", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hts-2")

		deviceName := "test-device"
		var mapSizeInMb = 3072
		var memorySizeInMb = mapSizeInMb / 10
		var diskSizeInMb = mapSizeInMb * 2
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()

		totalMemorySize := strconv.Itoa(memorySizeInMb*4) + "Mi"
		nativeMemorySize := strconv.Itoa(memorySizeInMb) + "Mi"
		diskSize := strconv.Itoa(diskSizeInMb) + "Mi"
		hazelcast := hazelcastconfig.HazelcastTieredStorage(hzLookupKey, deviceName, labels)
		hazelcast.Spec.LocalDevices[0].PVC.RequestStorage = &[]resource.Quantity{resource.MustParse(diskSize)}[0]
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(totalMemorySize)},
		}
		hazelcast.Spec.NativeMemory = &hazelcastv1alpha1.NativeMemoryConfiguration{
			Size: []resource.Quantity{resource.MustParse(nativeMemorySize)}[0],
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and putting entries")
		tsMap := hazelcastconfig.DefaultTieredStoreMap(mapLookupKey, hazelcast.Name, deviceName, labels)
		tsMap.Spec.TieredStore.MemoryRequestStorage = &[]resource.Quantity{resource.MustParse(nativeMemorySize)}[0]
		Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
		assertMapStatus(tsMap, hazelcastv1alpha1.MapSuccess)
		FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

		WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 30*Minute)
	})

})
