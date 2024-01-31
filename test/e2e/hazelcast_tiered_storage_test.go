package e2e

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Tiered Storage feature enabled", Label("hz_tiered_storage"), func() {

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

	It("should successfully fill the map with more than allocated memory", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hts-1")

		deviceName := "test-device"
		var mapSizeInMb = 3072
		var memorySizeInMb = mapSizeInMb / 10
		var diskSizeInMb = mapSizeInMb * 2
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()

		totalMemorySize := strconv.Itoa(memorySizeInMb*4) + "Mi"
		nativeMemorySize := strconv.Itoa(memorySizeInMb) + "Mi"
		diskSize := strconv.Itoa(diskSizeInMb) + "Mi"
		hazelcast := hazelcastconfig.HazelcastTieredStorage(hzLookupKey, deviceName, diskSize, labels)
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
		dm := hazelcastconfig.TieredStoreMap(mapLookupKey, hazelcast.Name, deviceName, nativeMemorySize, labels)
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastv1alpha1.MapSuccess)
		FillTheMapWithData(ctx, dm.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

		WaitForMapSize(context.Background(), hzLookupKey, dm.MapName(), expectedMapSize, 30*Minute)
	})

})
