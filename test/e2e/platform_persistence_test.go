package e2e

import (
	"context"
	"fmt"
	"log"
	"strconv"
	. "time"

	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Platform Persistence", Label("platform_persistence"), func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		Cleanup(context.Background())
		deletePVCs(hzLookupKey)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should successfully start after one member restart", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hps-1")
		ctx := context.Background()
		clusterSize := int32(3)

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		t := Now()
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		err := FillMapByEntryCount(ctx, hzLookupKey, true, m.MapName(), 100)
		Expect(err).To(BeNil())

		DeletePod(hazelcast.Name+"-2", 0, hzLookupKey)
		WaitForPodReady(hazelcast.Name+"-2", hzLookupKey, 1*Minute)
		evaluateReadyMembers(hzLookupKey)

		logs := InitLogs(t, hzLookupKey)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogs(logReader, 30*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		WaitForMapSize(ctx, hzLookupKey, m.MapName(), 100, 1*Minute)
	})

	It("should restore 3 GB data after planned shutdown", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hps-2")
		var mapSizeInMb = 3072
		var pvcSizeInMb = mapSizeInMb * 2 // Taking backup duplicates the used storage
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()
		clusterSize := int32(3)

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		CreateHazelcastCR(hazelcast)

		By("creating the map config and putting entries")
		dm := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)
		FillMapBySizeInMb(ctx, dm.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 20*Minute)

		By("putting entries after backup")
		err := FillMapByEntryCount(ctx, hzLookupKey, false, dm.MapName(), 111)
		Expect(err).To(BeNil())

		By("deleting Hazelcast cluster")
		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from the existing backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		WaitForMapSize(context.Background(), hzLookupKey, dm.MapName(), expectedMapSize, 30*Minute)
	})

	It("should not start repartitioning after one member restart", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hps-3")
		ctx := context.Background()
		clusterSize := int32(3)
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		persistenceStrategy := map[string]string{
			"hazelcast.persistence.auto.cluster.state.strategy": "FROZEN",
		}

		hazelcast.Spec.Properties = persistenceStrategy
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		m.GetManagedFields()
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		err := FillMapByEntryCount(ctx, hzLookupKey, true, m.Name, 100)
		Expect(err).To(BeNil())
		t := Now()
		DeletePod(hazelcast.Name+"-2", 10, hzLookupKey)
		evaluateReadyMembers(hzLookupKey)
		logs := InitLogs(t, hzLookupKey)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogs(logReader, 40*Second, logInterval).Should(MatchRegexp("newState=FROZEN"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).ShouldNot(MatchRegexp("Repartitioning cluster data. Migration tasks count"))
		test.EventuallyInLogs(logReader, 50*Second, logInterval).Should(MatchRegexp("newState=ACTIVE"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).ShouldNot(MatchRegexp("Repartitioning cluster data. Migration tasks count"))
		WaitForMapSize(context.Background(), hzLookupKey, m.Name, 100, 10*Minute)
	})

	It("should not start repartitioning after planned shutdown", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hps-4")
		ctx := context.Background()
		clusterSize := int32(3)
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		m.GetManagedFields()
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		err := FillMapByEntryCount(ctx, hzLookupKey, true, m.Name, 100)
		Expect(err).To(BeNil())

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 1*Minute)

		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}
		t := Now()
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)
		logs := InitLogs(t, hzLookupKey)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogs(logReader, 20*Second, logInterval).Should(MatchRegexp("cluster state: PASSIVE"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).Should(MatchRegexp("specifiedReplicaCount=3, readyReplicas=1"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).ShouldNot(MatchRegexp("Repartitioning cluster data. Migration tasks count"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).Should(MatchRegexp("specifiedReplicaCount=3, readyReplicas=3"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).Should(MatchRegexp("newState=ACTIVE"))
		test.EventuallyInLogs(logReader, 20*Second, logInterval).ShouldNot(MatchRegexp("Repartitioning cluster data. Migration tasks count"))
		WaitForMapSize(context.Background(), hzLookupKey, m.Name, 100, 10*Minute)
	})

	It("should persist SQL mappings", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hps-5")

		hazelcast := hazelcastconfig.HazelcastSQLPersistence(hzLookupKey, 1, labels)

		By("creating cluster with sql mapping persistance enabled")
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating sql mapping")
		createSQLMappingPortForward(context.Background(), hazelcast, localPort, hzLookupKey.Name)

		By("removing Hazelcast CR")
		RemoveHazelcastCR(hazelcast)

		By("recreating cluster")
		hazelcast = hazelcastconfig.HazelcastSQLPersistence(hzLookupKey, 1, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("checking the sql mappings")
		waitForSQLMappingsPortForward(context.Background(), hazelcast, localPort, hzLookupKey.Name, 1*Minute)
	})

	DescribeTable("Hazelcast", func(policyType hazelcastcomv1alpha1.DataRecoveryPolicyType, mapNameSuffix string) {
		setLabelAndCRName("hps-6")
		var mapSizeInMb = 500
		var pvcSizeInMb = 14500
		var numMaps = 28
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()
		clusterSize := int32(3)

		By("creating Hazelcast cluster with 7999 partition count and 14Gb in 28 maps")
		jvmArgs := []string{
			"-Dhazelcast.partition.count=7999",
		}
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.JVM = &hazelcastcomv1alpha1.JVMConfiguration{
			Args: jvmArgs,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = policyType

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and putting entries")
		ConcurrentlyCreateAndFillMultipleMapsByMb(ctx, numMaps, mapSizeInMb, mapNameSuffix, hazelcast)

		By("making rollout StatefulSet restart")
		err := RolloutRestart(ctx, hazelcast)
		if err != nil {
			log.Fatalf("Failed to perform rollout restart: %v", err)
		}

		By("checking HZ status after rollout sts restart")
		Eventually(func() hazelcastcomv1alpha1.Phase {
			err := k8sClient.Get(ctx, hzLookupKey, hazelcast)
			Expect(err).ToNot(HaveOccurred())
			return hazelcast.Status.Phase
		}, 10*Minute, interval).ShouldNot(Equal(hazelcastcomv1alpha1.Pending))

		By("checking map size after rollout sts restart")
		for i := 0; i < numMaps; i++ {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, mapNameSuffix), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			WaitForMapSize(ctx, hzLookupKey, m.MapName(), expectedMapSize, 5*Minute)
		}
	},
		Entry("should start with FULL_RECOVERY_ONLY, auto.cluster.state=true and auto-remove-stale-data=false", Serial, Tag(EE|AnyCloud), hazelcastcomv1alpha1.FullRecovery, "fr"),
		Entry("should start with PARTIAL_RECOVERY_MOST_RECENT, auto.cluster.state=true and auto-remove-stale-data=true", Serial, Tag(EE|AnyCloud), hazelcastcomv1alpha1.MostRecent, "pr"),
	)

})
