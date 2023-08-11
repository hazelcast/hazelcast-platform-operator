package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Persistence feature enabled", Label("hz_persistence"), func() {
	localPort := strconv.Itoa(8400 + GinkgoParallelProcess())
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
		DeleteAllOf(&hazelcastv1alpha1.CronHotBackup{}, &hazelcastv1alpha1.CronHotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.HotBackup{}, &hazelcastv1alpha1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Map{}, &hazelcastv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())

	})

	It("should successfully trigger HotBackup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-1")
		clusterSize := int32(3)

		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		By("checking the HotBackup creation sequence")
		logs := InitLogs(t, hzLookupKey)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogs(logReader, 15*Second, logInterval).
			Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
		test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
			Should(ContainElements(
				ContainSubstring("Starting new hot backup with sequence"),
				ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"),
				MatchRegexp(`(.*) Backup of hot restart store (.*?) finished in [0-9]* ms`)))

		assertHotBackupSuccess(hotBackup, 1*Minute)
	})

	DescribeTable("should trigger corresponding action when startupActionSpecified", Label("slow"),
		func(action hazelcastv1alpha1.PersistenceStartupAction, dataPolicy hazelcastv1alpha1.DataRecoveryPolicyType) {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("hp-2")
			clusterSize := int32(3)

			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating HotBackup CR")
			t := Now()
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
			assertHotBackupSuccess(hotBackup, 1*Minute)

			seq := GetBackupSequence(t, hzLookupKey)
			RemoveHazelcastCR(hazelcast)

			By("creating new Hazelcast cluster from existing backup with 2 members")
			baseDir := "/data/hot-restart/hot-backup/backup-" + seq

			hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.BaseDir = baseDir
			hazelcast.Spec.ClusterSize = &[]int32{2}[0]
			hazelcast.Spec.Persistence.DataRecoveryTimeout = 60
			hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = dataPolicy
			hazelcast.Spec.Persistence.StartupAction = action
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)
			assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		},
		Entry("ForceStart", Label("slow"), hazelcastv1alpha1.ForceStart, hazelcastv1alpha1.FullRecovery),
		Entry("PartialStart", Label("slow"), hazelcastv1alpha1.PartialStart, hazelcastv1alpha1.MostRecent),
	)

	It("should trigger multiple HotBackups with CronHotBackup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-3")

		By("creating cron hot backup")
		hbSpec := &hazelcastv1alpha1.HotBackupSpec{}
		chb := hazelcastconfig.CronHotBackup(hzLookupKey, "*/5 * * * * *", hbSpec, labels)
		Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())

		By("waiting cron hot backup two create two backups")
		Eventually(func() int {
			hbl := &hazelcastv1alpha1.HotBackupList{}
			err := k8sClient.List(context.Background(), hbl, client.InNamespace(chb.Namespace), client.MatchingLabels(labels))
			if err != nil {
				return 0
			}
			return len(hbl.Items)
		}, 11*Second, 1*Second).Should(Equal(2))

		By("deleting the cron hot backup")
		DeleteAllOf(&hazelcastv1alpha1.CronHotBackup{}, &hazelcastv1alpha1.CronHotBackupList{}, hzNamespace, labels)

		By("seeing hot backup CRs are also deleted")
		hbl := &hazelcastv1alpha1.HotBackupList{}
		Expect(k8sClient.List(context.Background(), hbl, client.InNamespace(chb.Namespace), client.MatchingLabels(labels))).Should(Succeed())
		Expect(len(hbl.Items)).To(Equal(0))
	})

	backupRestore := func(hazelcast *hazelcastv1alpha1.Hazelcast, hotBackup *hazelcastv1alpha1.HotBackup, useBucketConfig bool) {
		By("creating cluster with backup enabled")
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("triggering backup")
		t := Now()
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		By("checking if backup status is correct")
		assertCorrectBackupStatus(hotBackup, GetBackupSequence(t, hzLookupKey))

		By("adding new entries after backup")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 15)

		By("removing Hazelcast CR")
		RemoveHazelcastCR(hazelcast)

		By("creating cluster from backup")
		restoredHz := hazelcastconfig.HazelcastRestore(hazelcast, restoreConfig(hotBackup, useBucketConfig))
		CreateHazelcastCR(restoredHz)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(restoredHz, hazelcastv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), restoredHz, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), restoredHz, localPort, m.MapName(), 10, 1*Minute)
	}

	It("should successfully restore from LocalBackup-PVC-HotBackupResourceName", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-4")
		clusterSize := int32(3)

		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		backupRestore(hazelcast, hotBackup, false)
	})

	DescribeTable("Should successfully restore from ExternalBackup-PVC", Label("slow"), func(bucketURI, secretName string, useBucketConfig bool) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-6")

		By("creating cluster with backup enabled")
		clusterSize := int32(3)

		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		backupRestore(hazelcast, hotBackup, useBucketConfig)
	},
		Entry("using AWS S3 bucket HotBackupResourceName", Label("slow"), "s3://operator-e2e-external-backup", "br-secret-s3", false),
		Entry("using GCP bucket HotBackupResourceName", Label("slow"), "gs://operator-e2e-external-backup", "br-secret-gcp", false),
		Entry("using Azure bucket HotBackupResourceName", Label("slow"), "azblob://operator-e2e-external-backup", "br-secret-az", false),
		Entry("using GCP bucket restore from BucketConfig", Label("slow"), "gs://operator-e2e-external-backup", "br-secret-gcp", true),
	)

	It("should start HotBackup after cluster is ready", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-8")

		clusterSize := int32(1)
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastv1alpha1.MostRecent

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		assertHotBackupStatus(hotBackup, hazelcastv1alpha1.HotBackupPending, 1*Minute)

		By("creating cluster with backup enabled")
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		assertHotBackupSuccess(hotBackup, 1*Minute)
	})

	It("should persist SQL mappings", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-9")

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
})
