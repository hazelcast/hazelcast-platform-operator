package e2e

import (
	"context"
	"fmt"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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
			var nativeMemorySizeInMb = 1536 // 1.5Gi
			var mapMemory = 32
			var mapSizeInMb = 128
			var totalMemorySizeInMb = 2048 // 2 Gi
			var diskSizeInMb = 2048 * 2
			var expectedMapSize = int(float64(mapSizeInMb) * 128)
			ctx := context.Background()

			totalMemorySize := strconv.Itoa(totalMemorySizeInMb) + "Mi"
			nativeMemorySize := strconv.Itoa(nativeMemorySizeInMb) + "Mi"
			mapMemoryCapacitySize := strconv.Itoa(mapMemory) + "Mi"
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
			tsMap.Spec.TieredStore.MemoryCapacity = &[]resource.Quantity{resource.MustParse(mapMemoryCapacitySize)}[0]
			Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
			assertMapStatus(tsMap, hazelcastv1alpha1.MapSuccess)
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 30*Minute)
		})

		It("should fail to fill the map with more than allocated memory + disk size", Tag(EE|AnyCloud), func() {
			setLabelAndCRName("hpts-2")

			deviceName := "test-device"
			var nativeMemorySizeInMb = 1536 // 1.5Gi
			var mapMemory = 32
			var mapSizeInMb = 3000         // 3 Gi
			var totalMemorySizeInMb = 2048 // 2 Gi
			var diskSizeInMb = 1000
			ctx := context.Background()

			totalMemorySize := strconv.Itoa(totalMemorySizeInMb) + "Mi"
			nativeMemorySize := strconv.Itoa(nativeMemorySizeInMb) + "Mi"
			mapMemoryCapacitySize := strconv.Itoa(mapMemory) + "Mi"
			diskSize := strconv.Itoa(diskSizeInMb) + "Mi"
			hazelcast := hazelcastconfig.HazelcastTieredStorage(hzLookupKey, deviceName, labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(2)
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
			tsMap.Spec.TieredStore.MemoryCapacity = &[]resource.Quantity{resource.MustParse(mapMemoryCapacitySize)}[0]
			Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
			assertMapStatus(tsMap, hazelcastv1alpha1.MapSuccess)

			fmt.Printf("filling the map '%s' with '%d' MB data\n", tsMap.MapName(), mapSizeInMb)
			hzAddress := hzclient.HazelcastUrl(hazelcast)
			clientHz := GetHzClient(ctx, types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}, true)
			defer func() {
				err := clientHz.Shutdown(ctx)
				Expect(err).ToNot(HaveOccurred())
			}()
			t := Now()
			mapLoaderPod := createMapLoaderPod(hzAddress, hazelcast.Spec.ClusterName, mapSizeInMb, tsMap.MapName(), types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace})
			defer DeletePod(mapLoaderPod.Name, 10, types.NamespacedName{Namespace: hazelcast.Namespace})

			CheckPodStatus(types.NamespacedName{Name: mapLoaderPod.Name, Namespace: mapLoaderPod.Namespace}, corev1.PodFailed)

			logs := GetPodLogs(context.Background(), types.NamespacedName{
				Name:      mapLoaderPod.Name,
				Namespace: mapLoaderPod.Namespace,
			}, &corev1.PodLogOptions{
				Follow:    true,
				SinceTime: &metav1.Time{Time: t},
			})

			logReader := test.NewLogReader(logs)
			defer logReader.Close()
			test.EventuallyInLogs(logReader, 300*Second, logInterval/2).
				Should(ContainSubstring("com.hazelcast.internal.tstore.device.DeviceOutOfCapacityException"))
		})

		It("should get all data after scale down and up", Tag(EE|AnyCloud), func() {
			setLabelAndCRName("hpts-3")

			deviceName := "test-device"
			var nativeMemorySizeInMb = 1536 // 1.5Gi
			var mapMemory = 32
			var mapSizeInMb = 200
			var totalMemorySizeInMb = 2048 // 2 Gi
			var diskSizeInMb = 1024        // 1 Gi
			var expectedMapSize = int(float64(mapSizeInMb) * 128)
			ctx := context.Background()

			totalMemorySize := strconv.Itoa(totalMemorySizeInMb) + "Mi"
			nativeMemorySize := strconv.Itoa(nativeMemorySizeInMb) + "Mi"
			mapMemoryCapacitySize := strconv.Itoa(mapMemory) + "Mi"
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
			tsMap.Spec.TieredStore.MemoryCapacity = &[]resource.Quantity{resource.MustParse(mapMemoryCapacitySize)}[0]
			Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
			assertMapStatus(tsMap, hazelcastv1alpha1.MapSuccess)
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 3*Minute)

			By("scale down Hazelcast")
			UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
				hazelcast.Spec.ClusterSize = pointer.Int32(2)
				return hazelcast
			})
			WaitForReplicaSize(hazelcast.Namespace, hazelcast.Name, 2)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 3*Minute)

			By("scale up Hazelcast")
			UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
				hazelcast.Spec.ClusterSize = pointer.Int32(3)
				return hazelcast
			})
			evaluateReadyMembers(hzLookupKey)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 3*Minute)

		})

		It("should get all data and member should join the cluster after ungraceful shutdown", Tag(EE|AnyCloud), func() {
			setLabelAndCRName("hpts-4")

			deviceName := "test-device"
			var nativeMemorySizeInMb = 1536 // 1.5Gi
			var mapMemory = 32
			var mapSizeInMb = 200
			var totalMemorySizeInMb = 2048 // 2 Gi
			var diskSizeInMb = 1024        // 1 Gi
			var expectedMapSize = int(float64(mapSizeInMb) * 128)
			ctx := context.Background()

			totalMemorySize := strconv.Itoa(totalMemorySizeInMb) + "Mi"
			nativeMemorySize := strconv.Itoa(nativeMemorySizeInMb) + "Mi"
			mapMemoryCapacitySize := strconv.Itoa(mapMemory) + "Mi"
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
			tsMap.Spec.TieredStore.MemoryCapacity = &[]resource.Quantity{resource.MustParse(mapMemoryCapacitySize)}[0]
			Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
			assertMapStatus(tsMap, hazelcastv1alpha1.MapSuccess)
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 3*Minute)

			DeletePod(hazelcast.Name+"-2", 0, hzLookupKey)
			WaitForPodReady(hazelcast.Name+"-2", hzLookupKey, 1*Minute)
			evaluateReadyMembers(hzLookupKey)

			WaitForMapSize(context.Background(), hzLookupKey, tsMap.MapName(), expectedMapSize, 3*Minute)

		})
	})

})
