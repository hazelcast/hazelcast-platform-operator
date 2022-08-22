package e2e

import (
	"context"
	"math"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast WAN", Label("hz_wan_slow"), func() {
	waitForLBAddress := func(name types.NamespacedName) string {
		By("waiting for load balancer address")
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() string {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(context.Background(), name, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.ExternalAddresses
		}, 3*Minute, interval).Should(Not(BeEmpty()))
		Expect(k8sClient.Get(context.Background(), name, hz)).To(Succeed())
		return hz.Status.ExternalAddresses
	}

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
	})

	AfterEach(func() {
		for _, ns := range []string{sourceLookupKey.Namespace, targetLookupKey.Namespace} {
			DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, &hazelcastcomv1alpha1.WanReplicationList{}, ns, labels)
			DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, ns, labels)
			DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, ns, labels)
		}
	})

	It("should send 9 GB data by each cluster in active-passive mode in the different namespaces", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hwap-1")
		var mapSizeInGb = 3
		expectedTrgMapSize := int(float64(mapSizeInGb) * math.Round(1310.72) * 100)

		By("creating source Hazelcast cluster")
		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		evaluateReadyMembers(sourceLookupKey, 3)
		_ = waitForLBAddress(sourceLookupKey)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		evaluateReadyMembers(targetLookupKey, 3)
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
		FillTheMapWithHugeData(context.Background(), m.Name, mapSizeInGb, hazelcastSource)

		By("checking the target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, m.Name, expectedTrgMapSize)
	})

	It("should send 9 GB data by each cluster in active-active mode in the different namespaces", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		var mapSizeInGb = 3
		/**
		1310.72 (entries per single goroutine) = 1073741824 (Bytes per 1Gb)  / 8192 (Bytes per entry) / 100 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInGb) * math.Round(1310.72) * 100)
		expectedSrcMapSize := int(float64(mapSizeInGb*2) * math.Round(1310.72) * 100)
		setLabelAndCRName("hwaa-1")

		By("creating source Hazelcast cluster")
		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb*2) + "Gi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		sourceAddress := waitForLBAddress(sourceLookupKey)
		evaluateReadyMembers(sourceLookupKey, 3)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb*2) + "Gi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		targetAddress := waitForLBAddress(targetLookupKey)
		evaluateReadyMembers(targetLookupKey, 3)

		By("creating map for source Hazelcast cluster")
		mapSrc := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
		mapSrc.Spec.Name = "wanmap"
		Expect(k8sClient.Create(context.Background(), mapSrc)).Should(Succeed())
		mapSrc = assertMapStatus(mapSrc, hazelcastcomv1alpha1.MapSuccess)

		By("creating map for target Hazelcast cluster")
		mapTrg := hazelcastconfig.DefaultMap(targetLookupKey, hazelcastTarget.Name, labels)
		mapTrg.Spec.Name = "wanmap"
		Expect(k8sClient.Create(context.Background(), mapTrg)).Should(Succeed())
		mapTrg = assertMapStatus(mapTrg, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration for source Hazelcast cluster")
		wanSrc := hazelcastconfig.DefaultWanReplication(
			sourceLookupKey,
			mapSrc.Name,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
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
		wanTrg := hazelcastconfig.DefaultWanReplication(
			targetLookupKey,
			mapTrg.Name,
			hazelcastSource.Spec.ClusterName,
			sourceAddress,
			labels,
		)
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

		By("filling the source Map")
		FillTheMapWithHugeData(context.Background(), mapSrc.Spec.Name, mapSizeInGb, hazelcastSource)

		By("checking the target Map size")
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc.Spec.Name, expectedTrgMapSize)

		By("filling the target Map")
		FillTheMapWithHugeData(context.Background(), mapTrg.Spec.Name, mapSizeInGb, hazelcastTarget)

		By("checking the source Map size")
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg.Spec.Name, expectedSrcMapSize)
	})

	It("should send 9 GB data by each cluster in active-passive mode in the different GKE clusters", Serial, Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hwapdc-1")
		var mapSizeInGb = 3
		/**
		1310.72 (entries per single goroutine) = 1073741824 (Bytes per 1Gb)  / 8192 (Bytes per entry) / 100 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInGb) * math.Round(1310.72) * 100)

		By("creating source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		evaluateReadyMembers(sourceLookupKey, 3)
		_ = waitForLBAddress(sourceLookupKey)

		By("creating target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		evaluateReadyMembers(targetLookupKey, 3)
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
		FillTheMapWithHugeData(context.Background(), m.Name, mapSizeInGb, hazelcastSource)

		By("checking the target Map size")
		SwitchContext(context2)
		setupEnv()
		WaitForMapSize(context.Background(), targetLookupKey, m.Name, expectedTrgMapSize)
	})

	It("should send 9 GB data by each cluster in active-active mode in the different GKE clusters", Serial, Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		var mapSizeInGb = 3
		/**
		1310.72 (entries per single goroutine) = 1073741824 (Bytes per 1Gb)  / 8192 (Bytes per entry) / 100 (goroutines)
		*/
		expectedTrgMapSize := int(float64(mapSizeInGb) * math.Round(1310.72) * 100)
		expectedSrcMapSize := int(float64(mapSizeInGb*2) * math.Round(1310.72) * 100)

		setLabelAndCRName("hwaadc-1")

		By("creating source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		hazelcastSource := hazelcastconfig.ExposeExternallySmartLoadBalancer(sourceLookupKey, ee, labels)
		hazelcastSource.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb*2) + "Gi")},
		}
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		evaluateReadyMembers(sourceLookupKey, 3)
		sourceAddress := waitForLBAddress(sourceLookupKey)

		By("creating target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		hazelcastTarget := hazelcastconfig.ExposeExternallySmartLoadBalancer(targetLookupKey, ee, labels)
		hazelcastTarget.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb*2) + "Gi")},
		}
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		targetAddress := waitForLBAddress(targetLookupKey)
		evaluateReadyMembers(targetLookupKey, 3)

		By("Creating map for source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		mapSrc := hazelcastconfig.DefaultMap(sourceLookupKey, hazelcastSource.Name, labels)
		mapSrc.Spec.Name = "wanmap"
		Expect(k8sClient.Create(context.Background(), mapSrc)).Should(Succeed())
		mapSrc = assertMapStatus(mapSrc, hazelcastcomv1alpha1.MapSuccess)

		By("Creating map for target Hazelcast cluster")
		SwitchContext(context2)
		setupEnv()
		mapTrg := hazelcastconfig.DefaultMap(targetLookupKey, hazelcastTarget.Name, labels)
		mapTrg.Spec.Name = "wanmap"
		Expect(k8sClient.Create(context.Background(), mapTrg)).Should(Succeed())
		mapTrg = assertMapStatus(mapTrg, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration for source Hazelcast cluster")
		SwitchContext(context1)
		setupEnv()
		wanSrc := hazelcastconfig.DefaultWanReplication(
			sourceLookupKey,
			mapSrc.Name,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
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
		wanTrg := hazelcastconfig.DefaultWanReplication(
			targetLookupKey,
			mapTrg.Name,
			hazelcastSource.Spec.ClusterName,
			sourceAddress,
			labels,
		)
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

		By("filling the source Map")
		SwitchContext(context1)
		setupEnv()
		FillTheMapWithHugeData(context.Background(), mapSrc.Spec.Name, mapSizeInGb, hazelcastSource)

		By("checking the target Map size")
		SwitchContext(context2)
		setupEnv()
		WaitForMapSize(context.Background(), targetLookupKey, mapSrc.Spec.Name, expectedTrgMapSize)

		By("filling the target Map")
		FillTheMapWithHugeData(context.Background(), mapTrg.Spec.Name, mapSizeInGb, hazelcastTarget)

		By("checking the source Map size")
		SwitchContext(context1)
		setupEnv()
		WaitForMapSize(context.Background(), sourceLookupKey, mapTrg.Spec.Name, expectedSrcMapSize)
	})
})
