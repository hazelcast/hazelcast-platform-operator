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

var _ = Describe("Hazelcast CR with Tiered Storage feature enabled", Group("platform_tiered_storage"), func() {

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
		It("should successfully fill the map with more than allocated memory", Tag(EE|AnyCloud), func() {
			setLabelAndCRName("hpts-1")

			deviceName := "test-device"
			var memorySizeInMb = 1536      // 1.5Gi
			var mapSizeInMb = 2048         // 2 Gi
			var totalMemorySizeInMb = 2048 // 2 Gi
			var diskSizeInMb = mapSizeInMb * 2
			var expectedMapSize = int(float64(mapSizeInMb) * 128)
			ctx := context.Background()

			totalMemorySize := strconv.Itoa(totalMemorySizeInMb) + "Mi"
			nativeMemorySize := strconv.Itoa(memorySizeInMb) + "Mi"
			diskSize := strconv.Itoa(diskSizeInMb) + "Mi"
			hazelcast := hazelcastconfig.HazelcastTieredStorage(hzLookupKey, deviceName, labels)
			hazelcast.Spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			}
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
			tsMap.Spec.TieredStore.MemoryCapacity = &[]resource.Quantity{resource.MustParse(nativeMemorySize)}[0]
			Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
			assertMapStatus(tsMap, hazelcastv1alpha1.MapSuccess)
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 30*Minute)
		})
	})

})
