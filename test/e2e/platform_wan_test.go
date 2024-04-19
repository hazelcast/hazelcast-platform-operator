package e2e

import (
	"context"
	"fmt"
	chaosmeshv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"strconv"
	"strings"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast WAN", Label("platform_wan"), func() {
	waitForLBAddress := func(name types.NamespacedName) string {
		By("waiting for load balancer address")
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Expect(k8sClient.Get(context.Background(), name, hz)).ToNot(HaveOccurred())
		wanAddresses := fetchHazelcastAddressesByType(hz, hazelcastcomv1alpha1.HazelcastEndpointTypeWAN)
		return strings.Join(wanAddresses, ",")
	}

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		for _, ns := range []string{sourceLookupKey.Namespace, targetLookupKey.Namespace} {
			DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, &hazelcastcomv1alpha1.WanReplicationList{}, ns, labels)
			DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, ns, labels)
			DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, ns, labels)
		}
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should send 3 GB data by each cluster in active-passive mode in the different namespaces", Tag(EE|AnyCloud), func() {
		SwitchContext(context1)
		setupEnv()
		setLabelAndCRName("hpwan-1")
		var mapSizeInMb = 1024
		expectedTrgMapSize := int(float64(mapSizeInMb) * 128)

		By("creating source Hazelcast cluster")
		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*2) + "Mi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		evaluateReadyMembers(sourceLookupKey)
		_ = waitForLBAddress(sourceLookupKey)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*2) + "Mi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		evaluateReadyMembers(targetLookupKey)
		targetAddress := waitForLBAddress(targetLookupKey)

		By("creating map for source Hazelcast cluster")
		m := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			sourceLookupKey,
			m.Name,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
		wan.Spec.Queue.Capacity = 3000000
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wan := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), sourceLookupKey, wan)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wan.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		By("filling the Map")
		FillMapBySizeInMb(context.Background(), m.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, m.Name, expectedTrgMapSize, 30*Minute)
	})

	It("should send 6 GB data by each cluster in active-active mode in the different namespaces", Serial, Tag(EE|AnyCloud), func() {
		SwitchContext(context1)
		setupEnv()
		setLabelAndCRName("hpwan-2")
		var mapSizeInMb = 1024
		/**
		2 (entries per single goroutine) = 1048576  (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInMb) * 128)
		expectedSrcMapSize := int(float64(mapSizeInMb*2) * 128)

		By("creating source Hazelcast cluster")
		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		sourceAddress := waitForLBAddress(sourceLookupKey)
		evaluateReadyMembers(sourceLookupKey)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		targetAddress := waitForLBAddress(targetLookupKey)
		evaluateReadyMembers(targetLookupKey)

		By("creating first map for source Hazelcast cluster")
		mapSrc1 := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
		mapSrc1.Spec.Name = "wanmap1"
		Expect(k8sClient.Create(context.Background(), mapSrc1)).Should(Succeed())
		mapSrc1 = assertMapStatus(mapSrc1, hazelcastcomv1alpha1.MapSuccess)

		By("creating second map for source Hazelcast cluster")
		mapSrc2 := hazelcastconfig.DefaultMap(sourceLookupKey2, hazelcastSource.Name, labels)
		mapSrc2.Spec.Name = "wanmap2"
		Expect(k8sClient.Create(context.Background(), mapSrc2)).Should(Succeed())
		mapSrc2 = assertMapStatus(mapSrc2, hazelcastcomv1alpha1.MapSuccess)

		By("creating first map for target Hazelcast cluster")
		mapTrg1 := hazelcastconfig.DefaultMap(targetLookupKey, hazelcastTarget.Name, labels)
		mapTrg1.Spec.Name = "wanmap1"
		Expect(k8sClient.Create(context.Background(), mapTrg1)).Should(Succeed())
		mapTrg1 = assertMapStatus(mapTrg1, hazelcastcomv1alpha1.MapSuccess)

		By("creating second map for target Hazelcast cluster")
		mapTrg2 := hazelcastconfig.DefaultMap(targetLookupKey2, hazelcastTarget.Name, labels)
		mapTrg2.Spec.Name = "wanmap2"
		Expect(k8sClient.Create(context.Background(), mapTrg2)).Should(Succeed())
		mapTrg2 = assertMapStatus(mapTrg2, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration for source Hazelcast cluster")
		wanSrc := hazelcastconfig.CustomWanReplication(
			sourceLookupKey,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
		wanSrc.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
			Name: mapSrc1.Name,
			Kind: hazelcastcomv1alpha1.ResourceKindMap,
		},
			{
				Name: mapSrc2.Name,
				Kind: hazelcastcomv1alpha1.ResourceKindMap,
			}}
		wanSrc.Spec.Queue.Capacity = 3000000
		Expect(k8sClient.Create(context.Background(), wanSrc)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wanSrc := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), sourceLookupKey, wanSrc)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wanSrc.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		By("creating WAN configuration for target Hazelcast cluster")
		wanTrg := hazelcastconfig.CustomWanReplication(
			targetLookupKey,
			hazelcastSource.Spec.ClusterName,
			sourceAddress,
			labels,
		)
		wanTrg.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
			Name: hazelcastTarget.Name,
			Kind: hazelcastcomv1alpha1.ResourceKindHZ,
		}}
		wanTrg.Spec.Queue.Capacity = 3000000
		Expect(k8sClient.Create(context.Background(), wanTrg)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wanTrg := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), targetLookupKey, wanTrg)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wanTrg.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		By("filling the first source Map")
		FillMapBySizeInMb(context.Background(), mapSrc1.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("filling the second source Map")
		FillMapBySizeInMb(context.Background(), mapSrc2.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the first target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc1.MapName(), expectedTrgMapSize, 30*Minute)

		By("checking the second target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc2.MapName(), expectedTrgMapSize, 30*Minute)

		By("filling the first target Map")
		FillMapBySizeInMb(context.Background(), mapTrg1.MapName(), mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("filling the second target Map")
		FillMapBySizeInMb(context.Background(), mapTrg2.MapName(), mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("checking the first source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg1.MapName(), expectedSrcMapSize, 30*Minute)

		By("checking the second source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg2.MapName(), expectedSrcMapSize, 30*Minute)
	})

	It("should send 3 GB data by each cluster in active-passive mode in the different GKE clusters", Serial, Tag(EE|AnyCloud), func() {
		var mapSizeInMb = 1024
		/**
		2 (entries per single goroutine) = 1048576  (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInMb) * 128)

		By("creating source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		setLabelAndCRName("hpwan-3")

		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*2) + "Mi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		evaluateReadyMembers(sourceLookupKey)
		_ = waitForLBAddress(sourceLookupKey)

		By("creating target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*2) + "Mi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		evaluateReadyMembers(targetLookupKey)
		targetAddress := waitForLBAddress(targetLookupKey)

		By("Creating map for source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		m := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			sourceLookupKey,
			m.Name,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
		wan.Spec.Queue.Capacity = 3000000
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wan := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), sourceLookupKey, wan)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wan.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		By("filling the Map")
		FillMapBySizeInMb(context.Background(), m.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the target Map size")
		SwitchContext(context2)
		setupEnv()
		WaitForMapSize(context.Background(), targetLookupKey, m.Name, expectedTrgMapSize, 30*Minute)
	})

	It("should send 6 GB data by each cluster in active-active mode in the different GKE clusters", Serial, Tag(EE|AnyCloud), func() {
		var mapSizeInMb = 1024
		/**
		2 (entries per single goroutine) = 1048576  (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInMb) * 128)
		expectedSrcMapSize := int(float64(mapSizeInMb*2) * 128)

		By("creating source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		setLabelAndCRName("hpwan-4")

		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		evaluateReadyMembers(sourceLookupKey)
		sourceAddress := waitForLBAddress(sourceLookupKey)

		By("creating target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		targetAddress := waitForLBAddress(targetLookupKey)
		evaluateReadyMembers(targetLookupKey)

		By("creating first map for source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		mapSrc1 := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
		mapSrc1.Spec.Name = "wanmap1"
		Expect(k8sClient.Create(context.Background(), mapSrc1)).Should(Succeed())
		mapSrc1 = assertMapStatus(mapSrc1, hazelcastcomv1alpha1.MapSuccess)

		By("creating second map for source Hazelcast cluster")
		mapSrc2 := hazelcastconfig.DefaultMap(sourceLookupKey2, hazelcastSource.Name, labels)
		mapSrc2.Spec.Name = "wanmap2"
		Expect(k8sClient.Create(context.Background(), mapSrc2)).Should(Succeed())
		mapSrc2 = assertMapStatus(mapSrc2, hazelcastcomv1alpha1.MapSuccess)

		By("creating first map for target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		mapTrg1 := hazelcastconfig.DefaultMap(targetLookupKey, hazelcastTarget.Name, labels)
		mapTrg1.Spec.Name = "wanmap1"
		Expect(k8sClient.Create(context.Background(), mapTrg1)).Should(Succeed())
		mapTrg1 = assertMapStatus(mapTrg1, hazelcastcomv1alpha1.MapSuccess)

		By("creating second map for target Hazelcast cluster")
		mapTrg2 := hazelcastconfig.DefaultMap(targetLookupKey2, hazelcastTarget.Name, labels)
		mapTrg2.Spec.Name = "wanmap2"
		Expect(k8sClient.Create(context.Background(), mapTrg2)).Should(Succeed())
		mapTrg2 = assertMapStatus(mapTrg2, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration for source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		wanSrc := hazelcastconfig.CustomWanReplication(
			sourceLookupKey,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
		wanSrc.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
			Name: mapSrc1.Name,
			Kind: hazelcastcomv1alpha1.ResourceKindMap,
		},
			{
				Name: mapSrc2.Name,
				Kind: hazelcastcomv1alpha1.ResourceKindMap,
			}}
		wanSrc.Spec.Queue.Capacity = 3000000
		Expect(k8sClient.Create(context.Background(), wanSrc)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wanSrc := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), sourceLookupKey, wanSrc)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wanSrc.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		By("creating WAN configuration for target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		wanTrg := hazelcastconfig.CustomWanReplication(
			targetLookupKey,
			hazelcastSource.Spec.ClusterName,
			sourceAddress,
			labels,
		)
		wanTrg.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
			Name: hazelcastTarget.Name,
			Kind: hazelcastcomv1alpha1.ResourceKindHZ,
		}}
		wanTrg.Spec.Queue.Capacity = 3000000
		Expect(k8sClient.Create(context.Background(), wanTrg)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wanTrg := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), targetLookupKey, wanTrg)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wanTrg.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		By("filling the first source Map")
		SwitchContext(context1)
		setupEnv()
		FillMapBySizeInMb(context.Background(), mapSrc1.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("filling the second source Map")
		FillMapBySizeInMb(context.Background(), mapSrc2.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the first target Map size")
		SwitchContext(context2)
		setupEnv()
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc1.MapName(), expectedTrgMapSize, 30*Minute)

		By("checking the second target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc2.MapName(), expectedTrgMapSize, 30*Minute)

		By("filling the first target Map")
		FillMapBySizeInMb(context.Background(), mapTrg1.MapName(), mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("filling the second target Map")
		FillMapBySizeInMb(context.Background(), mapTrg2.MapName(), mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("checking the first source Map size")
		SwitchContext(context1)
		setupEnv()
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg1.Spec.Name, expectedSrcMapSize, 30*Minute)

		By("checking the second source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg2.Spec.Name, expectedSrcMapSize, 30*Minute)
	})

	Context("Data Sync between separate clusters", func() {
		It("should replicate data for maps with TS and non TS storage in active-passive WAN replication mode", Serial, Tag(EE|AnyCloud), func() {
			SwitchContext(context1)
			setupEnv()
			setLabelAndCRName("hpts-1")

			deviceName := "test-device"
			var nativeMemorySizeInMb = 1536 // 1.5Gi
			var mapMemory = 32
			var mapSizeInMb = 128
			var totalMemorySizeInMb = 2048 // 2 Gi
			var diskSizeInMb = 2048 * 2
			var expectedMapSize = int(float64(mapSizeInMb) * 128)
			ctx := context.Background()

			By("creating source Hazelcast cluster")
			totalMemorySize := strconv.Itoa(totalMemorySizeInMb) + "Mi"
			nativeMemorySize := strconv.Itoa(nativeMemorySizeInMb) + "Mi"
			mapMemoryCapacitySize := strconv.Itoa(mapMemory) + "Mi"
			diskSize := strconv.Itoa(diskSizeInMb) + "Mi"
			hazelcastSource := hazelcastconfig.HazelcastTieredStorage(sourceLookupKey, deviceName, labels)
			hazelcastSource.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			}
			hazelcastSource.Spec.LocalDevices[0].PVC.RequestStorage = &[]resource.Quantity{resource.MustParse(diskSize)}[0]
			hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(totalMemorySize)},
			}
			hazelcastSource.Spec.NativeMemory = &hazelcastcomv1alpha1.NativeMemoryConfiguration{
				Size: []resource.Quantity{resource.MustParse(nativeMemorySize)}[0],
			}

			CreateHazelcastCR(hazelcastSource)
			evaluateReadyMembers(sourceLookupKey)

			By("creating the TS map for source Hazelcast cluster")
			tsMap := hazelcastconfig.DefaultTieredStoreMap(sourceLookupKey, hazelcastSource.Name, deviceName, labels)
			tsMap.Spec.Name = "ts-map"
			tsMap.Spec.TieredStore.MemoryCapacity = &[]resource.Quantity{resource.MustParse(mapMemoryCapacitySize)}[0]
			Expect(k8sClient.Create(context.Background(), tsMap)).Should(Succeed())
			assertMapStatus(tsMap, hazelcastcomv1alpha1.MapSuccess)

			By("creating the non-TS map for source Hazelcast cluster")
			nonTsMap := hazelcastconfig.DefaultMap(sourceLookupKey2, hazelcastSource.Name, labels)
			nonTsMap.Spec.Name = "non-ts-map"
			Expect(k8sClient.Create(context.Background(), nonTsMap)).Should(Succeed())
			nonTsMap = assertMapStatus(nonTsMap, hazelcastcomv1alpha1.MapSuccess)

			By("creating target Hazelcast cluster")
			SwitchContext(context2)
			setupEnv()
			hazelcastTarget := hazelcastconfig.HazelcastTieredStorage(targetLookupKey, deviceName, labels)
			hazelcastTarget.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			}
			hazelcastTarget.Spec.LocalDevices[0].PVC.RequestStorage = &[]resource.Quantity{resource.MustParse(diskSize)}[0]
			hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(totalMemorySize)},
			}
			hazelcastTarget.Spec.NativeMemory = &hazelcastcomv1alpha1.NativeMemoryConfiguration{
				Size: []resource.Quantity{resource.MustParse(nativeMemorySize)}[0],
			}
			hazelcastTarget.Spec.ClusterName = "target"
			CreateHazelcastCR(hazelcastTarget)
			evaluateReadyMembers(targetLookupKey)
			targetAddress := waitForLBAddress(targetLookupKey)

			By("creating WAN configuration for source Hazelcast cluster")
			SwitchContext(context1)
			setupEnv()
			wanSrc := hazelcastconfig.CustomWanReplication(
				sourceLookupKey,
				hazelcastTarget.Spec.ClusterName,
				targetAddress,
				labels,
			)
			wanSrc.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
				Name: nonTsMap.Name,
				Kind: hazelcastcomv1alpha1.ResourceKindMap,
			},
				{
					Name: tsMap.Name,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				}}

			Expect(k8sClient.Create(context.Background(), wanSrc)).Should(Succeed())

			Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
				wanSrc := &hazelcastcomv1alpha1.WanReplication{}
				err := k8sClient.Get(context.Background(), sourceLookupKey, wanSrc)
				if err != nil {
					return hazelcastcomv1alpha1.WanStatusFailed, err
				}
				return wanSrc.Status.Status, nil
			}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

			By("1-st fill of the the TS Map and source cluster")
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

			By("1-st fill of the the non-TS Map and source cluster")
			FillMapBySizeInMb(context.Background(), nonTsMap.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

			SwitchContext(context2)
			setupEnv()
			By("checking the target TS map size after the 1-st fill")
			WaitForMapSize(ctx, targetLookupKey, tsMap.MapName(), expectedMapSize, 15*Minute)

			By("checking the target non-TS map size after the 1-st fill")
			WaitForMapSize(ctx, targetLookupKey, nonTsMap.MapName(), expectedMapSize, 15*Minute)

			By("update to wrong Hazelcast image")
			UpdateHazelcastCR(hazelcastTarget, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				hazelcast.Spec.Repository = "docker.io/hazelcast/hazelcast-enterprise-wrong"
				return hazelcast
			})
			DeletePod(hazelcastTarget.Name+"-0", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-1", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-2", 0, targetLookupKey)

			By("2-nd fill of the TS Map for the source cluster")
			SwitchContext(context1)
			setupEnv()
			FillMapBySizeInMb(ctx, nonTsMap.MapName(), mapSizeInMb, mapSizeInMb*2, hazelcastSource)

			By("2-nd fill of the TS Map for the source cluster")
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb*2, hazelcastSource)

			By("update to correct Hazelcast image")
			SwitchContext(context2)
			setupEnv()
			UpdateHazelcastCR(hazelcastTarget, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				hazelcast.Spec.Repository = "docker.io/hazelcast/hazelcast-enterprise"
				return hazelcast
			})
			DeletePod(hazelcastTarget.Name+"-0", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-1", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-2", 0, targetLookupKey)
			evaluateReadyMembers(targetLookupKey)

			By("trigger WAN sync")
			SwitchContext(context1)
			setupEnv()
			createWanSync(ctx, sourceLookupKey, wanSrc.Name, 2, labels)

			By("3-rd fill of the non TS Map for the source cluster")
			FillMapBySizeInMb(ctx, nonTsMap.MapName(), mapSizeInMb, mapSizeInMb*3, hazelcastSource)

			By("3-rd fill of the TS Map for the source cluster")
			FillMapBySizeInMb(ctx, tsMap.MapName(), mapSizeInMb, mapSizeInMb*3, hazelcastSource)

			SwitchContext(context2)
			setupEnv()
			By("checking the overall target TS map size")
			WaitForMapSize(ctx, targetLookupKey, tsMap.MapName(), expectedMapSize*3, 15*Minute)

			By("checking the overall target non-TS map size")
			WaitForMapSize(ctx, targetLookupKey, nonTsMap.MapName(), expectedMapSize*3, 15*Minute)
		})

		It("shouldn't fail in active-passive mode across separate clusters", Serial, Tag(EE|AnyCloud), func() {
			var mapSizeInMb = 5000
			expectedTrgMapSize := int(float64(mapSizeInMb) * 128)

			By("creating source Hazelcast cluster")
			SwitchContext(context1)
			setupEnv()
			setLabelAndCRName("hpwans-6")

			hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
			hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
			}
			hazelcastSource.Spec.ClusterName = "source"
			hazelcastSource.Spec.ClusterSize = pointer.Int32(3)
			CreateHazelcastCR(hazelcastSource)
			evaluateReadyMembers(sourceLookupKey)

			By("creating first map for source Hazelcast cluster")
			mapSrc1 := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
			mapSrc1.Spec.Name = "wanmap1"
			Expect(k8sClient.Create(context.Background(), mapSrc1)).Should(Succeed())
			mapSrc1 = assertMapStatus(mapSrc1, hazelcastcomv1alpha1.MapSuccess)

			By("creating second map for source Hazelcast cluster")
			mapSrc2 := hazelcastconfig.DefaultMap(sourceLookupKey2, hazelcastSource.Name, labels)
			mapSrc2.Spec.Name = "wanmap2"
			Expect(k8sClient.Create(context.Background(), mapSrc2)).Should(Succeed())
			mapSrc2 = assertMapStatus(mapSrc2, hazelcastcomv1alpha1.MapSuccess)

			By("creating target Hazelcast cluster")
			SwitchContext(context2)
			setupEnv()
			hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
			hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
			}
			hazelcastTarget.Spec.ClusterName = "target"
			hazelcastTarget.Spec.ClusterSize = pointer.Int32(3)
			CreateHazelcastCR(hazelcastTarget)
			targetAddress := waitForLBAddress(targetLookupKey)

			By("creating WAN configuration for source Hazelcast cluster")
			SwitchContext(context1)
			setupEnv()
			wanSrc := hazelcastconfig.CustomWanReplication(
				sourceLookupKey,
				hazelcastTarget.Spec.ClusterName,
				targetAddress,
				labels,
			)
			wanSrc.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
				Name: mapSrc1.Name,
				Kind: hazelcastcomv1alpha1.ResourceKindMap,
			},
				{
					Name: mapSrc2.Name,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				}}
			wanSrc.Spec.Queue.Capacity = 300000
			Expect(k8sClient.Create(context.Background(), wanSrc)).Should(Succeed())

			Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
				wanSrc := &hazelcastcomv1alpha1.WanReplication{}
				err := k8sClient.Get(context.Background(), sourceLookupKey, wanSrc)
				if err != nil {
					return hazelcastcomv1alpha1.WanStatusFailed, err
				}
				return wanSrc.Status.Status, nil
			}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

			By("update to wrong Hazelcast image")
			SwitchContext(context2)
			setupEnv()
			UpdateHazelcastCR(hazelcastTarget, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				hazelcast.Spec.Repository = "docker.io/hazelcast/hazelcast-enterprise2"
				return hazelcast
			})
			DeletePod(hazelcastTarget.Name+"-0", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-1", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-2", 0, targetLookupKey)

			By("filling the first source Map")
			SwitchContext(context1)
			setupEnv()
			FillMapBySizeInMb(context.Background(), mapSrc1.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

			By("filling the second source Map")
			FillMapBySizeInMb(context.Background(), mapSrc2.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

			By("update to correct Hazelcast image")
			SwitchContext(context2)
			setupEnv()
			UpdateHazelcastCR(hazelcastTarget, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				hazelcast.Spec.Repository = "docker.io/hazelcast/hazelcast-enterprise"
				return hazelcast
			})
			DeletePod(hazelcastTarget.Name+"-0", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-1", 0, targetLookupKey)
			DeletePod(hazelcastTarget.Name+"-2", 0, targetLookupKey)

			By("checking HZ status after update")
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), targetLookupKey, hazelcastTarget)
				Expect(err).ToNot(HaveOccurred())
				return hazelcastTarget.Status.Phase
			}, 5*Minute, interval).ShouldNot(Equal(hazelcastcomv1alpha1.Pending))

			By("trigger WAN sync")
			SwitchContext(context1)
			setupEnv()
			createWanSync(context.Background(), sourceLookupKey, wanSrc.Name, 2, labels)

			By("checking the first target Map size")
			SwitchContext(context2)
			setupEnv()
			WaitForMapSize(context.Background(), targetLookupKey, mapSrc1.Spec.Name, expectedTrgMapSize, 15*Minute)

			By("checking the second target Map size")
			WaitForMapSize(context.Background(), targetLookupKey, mapSrc2.Spec.Name, expectedTrgMapSize, 15*Minute)
		})

		It("shouldn't fail due to split brain in active-passive mode across separate clusters", Serial, Tag(EE|AnyCloud), func() {
			var mapSizeInMb = 1024
			duration := "3m"
			expectedTrgMapSize := int(float64(mapSizeInMb) * 128)

			By("creating source Hazelcast cluster")
			SwitchContext(context1)
			setupEnv()
			setLabelAndCRName("hpwans-7")

			hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
			hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
			}
			hazelcastSource.Spec.ClusterName = "source"
			hazelcastSource.Spec.ClusterSize = pointer.Int32(6)
			CreateHazelcastCR(hazelcastSource)
			evaluateReadyMembers(sourceLookupKey)

			By("creating non-TS map for source Hazelcast cluster")
			mapSrc1 := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
			mapSrc1.Spec.Name = "wanmap1"
			Expect(k8sClient.Create(context.Background(), mapSrc1)).Should(Succeed())
			mapSrc1 = assertMapStatus(mapSrc1, hazelcastcomv1alpha1.MapSuccess)

			By("creating second map for source Hazelcast cluster")
			mapSrc2 := hazelcastconfig.DefaultMap(sourceLookupKey2, hazelcastSource.Name, labels)
			mapSrc2.Spec.Name = "wanmap2"
			Expect(k8sClient.Create(context.Background(), mapSrc2)).Should(Succeed())
			mapSrc2 = assertMapStatus(mapSrc2, hazelcastcomv1alpha1.MapSuccess)

			By("creating target Hazelcast cluster")
			SwitchContext(context2)
			setupEnv()
			hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
			hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInMb*4) + "Mi")},
			}
			hazelcastTarget.Spec.ClusterName = "target"
			hazelcastTarget.Spec.ClusterSize = pointer.Int32(6)
			CreateHazelcastCR(hazelcastTarget)
			targetAddress := waitForLBAddress(targetLookupKey)

			By("split the members into 2 groups")
			podLabels := []PodLabel{
				{"statefulset.kubernetes.io/pod-name=" + hazelcastTarget.Name + "-0", "group", "group1"},
				{"statefulset.kubernetes.io/pod-name=" + hazelcastTarget.Name + "-1", "group", "group1"},
				{"statefulset.kubernetes.io/pod-name=" + hazelcastTarget.Name + "-2", "group", "group1"},
				{"statefulset.kubernetes.io/pod-name=" + hazelcastTarget.Name + "-3", "group", "group2"},
				{"statefulset.kubernetes.io/pod-name=" + hazelcastTarget.Name + "-4", "group", "group2"},
				{"statefulset.kubernetes.io/pod-name=" + hazelcastTarget.Name + "-5", "group", "group2"},
			}
			err := LabelPods(hazelcastTarget.Namespace, podLabels)
			if err != nil {
				fmt.Printf("Error labeling pods: %s\n", err)
			} else {
				fmt.Println("Pods labeled successfully")
			}
			evaluateReadyMembers(targetLookupKey)

			By("run split brain scenario")
			networkPartitionChaos := &chaosmeshv1alpha1.NetworkChaos{
				ObjectMeta: metav1.ObjectMeta{
					Name:      hazelcastTarget.Name,
					Namespace: hazelcastTarget.Namespace,
				},
				Spec: chaosmeshv1alpha1.NetworkChaosSpec{
					PodSelector: chaosmeshv1alpha1.PodSelector{
						Mode: chaosmeshv1alpha1.AllMode,
						Selector: chaosmeshv1alpha1.PodSelectorSpec{
							GenericSelectorSpec: chaosmeshv1alpha1.GenericSelectorSpec{
								LabelSelectors: map[string]string{
									"group": "group1",
								},
							},
						},
					},
					Action:    chaosmeshv1alpha1.PartitionAction,
					Direction: chaosmeshv1alpha1.Both,
					Target: &chaosmeshv1alpha1.PodSelector{
						Mode: chaosmeshv1alpha1.AllMode,
						Selector: chaosmeshv1alpha1.PodSelectorSpec{
							GenericSelectorSpec: chaosmeshv1alpha1.GenericSelectorSpec{
								LabelSelectors: map[string]string{
									"group": "group2",
								},
							},
						},
					},
					Duration: &duration,
				},
			}
			Expect(k8sClient.Create(context.Background(), networkPartitionChaos)).To(Succeed(), "Failed to create network partition chaos")

			By("wait until Hazelcast cluster will be injected by split-brain experiment")
			Eventually(func() bool {
				err = k8sClient.Get(context.Background(), targetLookupKey, networkPartitionChaos)
				Expect(err).ToNot(HaveOccurred())
				for _, condition := range networkPartitionChaos.Status.ChaosStatus.Conditions {
					if condition.Type == chaosmeshv1alpha1.ConditionAllInjected && condition.Status == "True" {
						return true
					}
				}
				return false
			}, 1*Minute, interval).Should(BeTrue())

			By("creating WAN configuration for source Hazelcast cluster")
			SwitchContext(context1)
			setupEnv()
			wanSrc := hazelcastconfig.CustomWanReplication(
				sourceLookupKey,
				hazelcastTarget.Spec.ClusterName,
				targetAddress,
				labels,
			)
			wanSrc.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{{
				Name: mapSrc1.Name,
				Kind: hazelcastcomv1alpha1.ResourceKindMap,
			},
				{
					Name: mapSrc2.Name,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				}}
			wanSrc.Spec.Queue.Capacity = 300000
			Expect(k8sClient.Create(context.Background(), wanSrc)).Should(Succeed())

			Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
				wanSrc := &hazelcastcomv1alpha1.WanReplication{}
				err := k8sClient.Get(context.Background(), sourceLookupKey, wanSrc)
				if err != nil {
					return hazelcastcomv1alpha1.WanStatusFailed, err
				}
				return wanSrc.Status.Status, nil
			}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

			By("filling the first source Map")
			FillMapBySizeInMb(context.Background(), mapSrc1.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

			By("filling the second source Map")
			FillMapBySizeInMb(context.Background(), mapSrc2.MapName(), mapSizeInMb, mapSizeInMb, hazelcastSource)

			By("trigger WAN sync")
			createWanSync(context.Background(), sourceLookupKey, wanSrc.Name, 2, labels)

			By("checking the first target Map size")
			SwitchContext(context2)
			setupEnv()
			WaitForMapSize(context.Background(), targetLookupKey, mapSrc1.MapName(), expectedTrgMapSize, 15*Minute)

			By("checking the second target Map size")
			WaitForMapSize(context.Background(), targetLookupKey, mapSrc2.MapName(), expectedTrgMapSize, 15*Minute)
		})

	})
})
