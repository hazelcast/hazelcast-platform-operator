package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Backup", Label("backup_slow"), func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1beta1.HotBackup{}, &hazelcastv1beta1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1beta1.Map{}, &hazelcastv1beta1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1beta1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1beta1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should successfully start after one member restart", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hbs-1")
		ctx := context.Background()
		clusterSize := int32(3)

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastv1beta1.MostRecent
		t := Now()
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1beta1.MapSuccess)

		FillTheMapData(ctx, hzLookupKey, true, m.MapName(), 100)

		DeletePod(hazelcast.Name+"-2", 0, hzLookupKey)
		WaitForPodReady(hazelcast.Name+"-2", hzLookupKey, 1*Minute)
		evaluateReadyMembers(hzLookupKey)

		logs := InitLogs(t, hzLookupKey)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogs(logReader, 30*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		WaitForMapSize(ctx, hzLookupKey, m.MapName(), 100, 1*Minute)
	})

	It("should restore 3 GB data after planned shutdown", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hbs-2")
		var mapSizeInMb = 3072
		var pvcSizeInMb = mapSizeInMb * 2 // Taking backup duplicates the used storage
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()
		clusterSize := int32(3)

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		CreateHazelcastCR(hazelcast)

		By("creating the map config and putting entries")
		dm := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastv1beta1.MapSuccess)
		FillTheMapWithData(ctx, dm.MapName(), mapSizeInMb, hazelcast)

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 20*Minute)

		By("putting entries after backup")
		FillTheMapData(ctx, hzLookupKey, false, dm.MapName(), 111)

		By("deleting Hazelcast cluster")
		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from the existing backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = hazelcastv1beta1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1beta1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		WaitForMapSize(context.Background(), hzLookupKey, dm.MapName(), expectedMapSize, 30*Minute)
	})

	It("Should successfully restore 3 Gb data from external backup using GCP bucket", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hbs-3")

		ctx := context.Background()
		var mapSizeInMb = 3072
		var pvcSizeInMb = mapSizeInMb * 2 // Taking backup duplicates the used storage
		var bucketURI = "gs://operator-e2e-external-backup"
		var secretName = "br-secret-gcp"
		expectedMapSize := int(float64(mapSizeInMb) * 128)
		clusterSize := int32(3)

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		dm := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastv1beta1.MapSuccess)

		By("filling the Map")
		FillTheMapWithData(ctx, dm.MapName(), mapSizeInMb, hazelcast)

		By("triggering the backup")
		hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 20*Minute)

		By("putting entries after backup")
		FillTheMapData(ctx, hzLookupKey, false, dm.MapName(), 111)

		By("deleting Hazelcast cluster")
		RemoveHazelcastCR(hazelcast)
		deletePVCs(hzLookupKey)

		By("creating cluster from external backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = hazelcastv1beta1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name}
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1beta1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		WaitForMapSize(context.Background(), hzLookupKey, dm.MapName(), expectedMapSize, 30*Minute)
	})

	It("should interrupt external backup process when the hotbackup is deleted", Label("slow"), func() {
		setLabelAndCRName("hbs-4")
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		ctx := context.Background()
		bucketURI := "gs://operator-e2e-external-backup"
		secretName := "br-secret-gcp"
		mapSizeInMb := 1024
		pvcSizeInMb := mapSizeInMb * 2 // Taking backup duplicates the used storage
		clusterSize := int32(3)

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1beta1.MapSuccess)

		By("filling the Map")
		FillTheMapWithData(ctx, m.MapName(), mapSizeInMb, hazelcast)

		t := Now()

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(ctx, hotBackup)).Should(Succeed())

		By("checking hazelcast logs if backup started")
		hzLogs := InitLogs(t, hzLookupKey)
		hzLogReader := test.NewLogReader(hzLogs)
		defer hzLogReader.Close()
		test.EventuallyInLogs(hzLogReader, 10*Second, logInterval).Should(ContainSubstring("Starting new hot backup with sequence"))
		test.EventuallyInLogs(hzLogReader, 10*Second, logInterval).Should(MatchRegexp(`Backup of hot restart store (.*?) finished in [0-9]* ms`))

		By("checking agent logs if upload is started")
		agentLogs := SidecarAgentLogs(t, hzLookupKey)
		agentLogReader := test.NewLogReader(agentLogs)
		defer agentLogReader.Close()
		test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("Starting new task"))
		test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("task is started"))
		test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("task successfully read secret"))
		test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("task is in progress"))

		By("get hotbackup object")
		hb := &hazelcastv1beta1.HotBackup{}
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
		Expect(err).ToNot(HaveOccurred())
		Expect(hb.Status.State).Should(Equal(hazelcastv1beta1.HotBackupInProgress))

		By("delete hotbackup to cancel backup process")
		err = k8sClient.Delete(ctx, hb)
		Expect(err).ToNot(HaveOccurred())

		By("checking agent logs if upload canceled")
		test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("canceling task"))
	})

	It("Should successfully restore multiple times from HotBackupResourceName", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-5")
		clusterSize := int32(3)

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1beta1.MapSuccess)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("triggering first backup as external")
		hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, "", "")
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("triggering second backup as local")
		hbLookupKey2 := types.NamespacedName{Name: hbLookupKey.Name + "2", Namespace: hbLookupKey.Namespace}
		hotBackup2 := hazelcastconfig.HotBackupBucket(hbLookupKey2, hazelcast.Name, labels, "gs://operator-e2e-external-backup", "br-secret-gcp")
		Expect(k8sClient.Create(context.Background(), hotBackup2)).Should(Succeed())
		hotBackup2 = assertHotBackupSuccess(hotBackup2, 1*Minute)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		RemoveHazelcastCR(hazelcast)

		By("creating cluster from from first backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = hazelcastv1beta1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1beta1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 10, 1*Minute)

		RemoveHazelcastCR(hazelcast)

		By("creating cluster from from second backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = hazelcastv1beta1.RestoreConfiguration{
			HotBackupResourceName: hotBackup2.Name,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1beta1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 20, 1*Minute)

	})

	It("should not start repartitioning after one member restart", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("rep-1")
		ctx := context.Background()
		clusterSize := int32(3)
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		persistenceStrategy := map[string]string{
			"hazelcast.persistence.auto.cluster.state.strategy": "FROZEN",
		}

		hazelcast.Spec.Properties = persistenceStrategy
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastv1beta1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		m.GetManagedFields()
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1beta1.MapSuccess)

		FillTheMapData(ctx, hzLookupKey, true, m.Name, 100)
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

	It("should not start repartitioning after planned shutdown", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("rep-2")
		ctx := context.Background()
		clusterSize := int32(3)
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastv1beta1.MostRecent

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		m.GetManagedFields()
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1beta1.MapSuccess)

		FillTheMapData(ctx, hzLookupKey, true, m.Name, 100)

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 1*Minute)

		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastv1beta1.ExposeExternallyConfiguration{
			Type:                 hazelcastv1beta1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastv1beta1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.Restore = hazelcastv1beta1.RestoreConfiguration{
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
})
