package e2e

import (
	"context"
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
		Cleanup(context.Background())
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
		FillMapBySizeInMb(context.Background(), m.Name, mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, m.Name, expectedTrgMapSize, 30*Minute)
	})

	It("should send 6 GB data by each cluster in active-active mode in the different namespaces", Tag(EE|AnyCloud), func() {
		SwitchContext(context1)
		setupEnv()
		var mapSizeInMb = 1024
		/**
		2 (entries per single goroutine) = 1048576  (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInMb) * 128)
		expectedSrcMapSize := int(float64(mapSizeInMb*2) * 128)
		setLabelAndCRName("hpwan-2")

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
		FillMapBySizeInMb(context.Background(), mapSrc1.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("filling the second source Map")
		FillMapBySizeInMb(context.Background(), mapSrc2.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the first target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc1.Spec.Name, expectedTrgMapSize, 30*Minute)

		By("checking the second target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc2.Spec.Name, expectedTrgMapSize, 30*Minute)

		By("filling the first target Map")
		FillMapBySizeInMb(context.Background(), mapTrg1.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("filling the second target Map")
		FillMapBySizeInMb(context.Background(), mapTrg2.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("checking the first source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg1.Spec.Name, expectedSrcMapSize, 30*Minute)

		By("checking the second source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg2.Spec.Name, expectedSrcMapSize, 30*Minute)
	})

	It("should send 3 GB data by each cluster in active-passive mode in the different GKE clusters", Serial, Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hpwan-3")
		var mapSizeInMb = 1024
		/**
		2 (entries per single goroutine) = 1048576  (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInMb) * 128)

		By("creating source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
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
		FillMapBySizeInMb(context.Background(), m.Name, mapSizeInMb, mapSizeInMb, hazelcastSource)

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

		setLabelAndCRName("hpwan-4")

		By("creating source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
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
		FillMapBySizeInMb(context.Background(), mapSrc1.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("filling the second source Map")
		FillMapBySizeInMb(context.Background(), mapSrc2.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastSource)

		By("checking the first target Map size")
		SwitchContext(context2)
		setupEnv()
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc1.Spec.Name, expectedTrgMapSize, 30*Minute)

		By("checking the second target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc2.Spec.Name, expectedTrgMapSize, 30*Minute)

		By("filling the first target Map")
		FillMapBySizeInMb(context.Background(), mapTrg1.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("filling the second target Map")
		FillMapBySizeInMb(context.Background(), mapTrg2.Spec.Name, mapSizeInMb, mapSizeInMb, hazelcastTarget)

		By("checking the first source Map size")
		SwitchContext(context1)
		setupEnv()
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg1.Spec.Name, expectedSrcMapSize, 30*Minute)

		By("checking the second source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg2.Spec.Name, expectedSrcMapSize, 30*Minute)
	})
})
