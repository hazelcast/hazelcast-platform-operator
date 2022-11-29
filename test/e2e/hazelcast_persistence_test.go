package e2e

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
		By("checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
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

		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels)
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		By("checking the HotBackup creation sequence")
		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
		test.EventuallyInLogsUnordered(scanner, 15*Second, logInterval).
			Should(ContainElements(
				ContainSubstring("Starting new hot backup with sequence"),
				ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"),
				MatchRegexp(`(.*) Backup of hot restart store (.*?) finished in [0-9]* ms`)))

		Expect(logs.Close()).Should(Succeed())

		assertHotBackupSuccess(hotBackup, 1*Minute)
	})

	It("should trigger ForceStart when restart from HotBackup failed", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-2")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels, false)
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
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, false)
		hazelcast.Spec.ClusterSize = &[]int32{2}[0]
		hazelcast.Spec.Persistence.DataRecoveryTimeout = 60
		hazelcast.Spec.Persistence.AutoForceStart = true
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
	})

	DescribeTable("should successfully restore from local backup using HotBackupResourceName", func(params ...interface{}) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-3")
		baseDir := "/data/hot-restart"
		hazelcast := addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, params...), getFirstWorkerNodeName())
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		By("adding new entries after backup")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 15)

		By("checking if backup status is correct")
		seq := GetBackupSequence(t, hzLookupKey)
		backupSeqFolder := hotBackup.Status.GetBackupFolder()
		Expect("backup-" + seq).Should(Equal(backupSeqFolder))

		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		hazelcast = addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, params...), getFirstWorkerNodeName())
		hazelcast.Spec.Persistence.Restore = &hazelcastv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey)

		assertHazelcastRestoreStatus(hazelcast, hazelcastv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 10, 1*Minute)

	},
		Entry("with PVC configuration", Label("slow")),
		//Entry("with HostPath configuration single node", Label("slow"), "/tmp/hazelcast/singleNode", "dummyNodeName"),
		//Entry("with HostPath configuration multiple nodes", Label("slow"), "/tmp/hazelcast/multiNode"),
	)

	DescribeTable("Should successfully restore from external backup", func(bucketURI, secretName string) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-4")

		By("creating cluster with backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("triggering backup")
		t := Now()
		hotBackup := hazelcastconfig.HotBackupAgent(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)
		backupBucketURI := hotBackup.Status.GetBucketURI()

		By("adding new entries after backup")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 15)

		By("checking if backup status is correct")
		seq := GetBackupSequence(t, hzLookupKey)
		timestamp, _ := strconv.ParseInt(seq, 10, 64)
		bucketURI += fmt.Sprintf("?prefix=%s/%s/", hzLookupKey.Name,
			unixMilli(timestamp).UTC().Format("2006-01-02-15-04-05")) // hazelcast/2022-06-02-21-57-49/
		Expect(bucketURI).Should(Equal(backupBucketURI))

		RemoveHazelcastCR(hazelcast)
		deletePVCs(hzLookupKey)

		By("creating cluster from external backup")
		hazelcast = hazelcastconfig.HazelcastRestore(hzLookupKey,
			&hazelcastv1alpha1.RestoreConfiguration{
				BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
					BucketURI: bucketURI,
					Secret:    secretName,
				},
			}, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		assertHazelcastRestoreStatus(hazelcast, hazelcastv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 10, 1*Minute)

	},
		Entry("using AWS S3 bucket", Label("slow"), "s3://operator-e2e-external-backup", "br-secret-s3"),
		Entry("using GCP bucket", Label("slow"), "gs://operator-e2e-external-backup", "br-secret-gcp"),
		Entry("using Azure bucket", Label("slow"), "azblob://operator-e2e-external-backup", "br-secret-az"),
	)

	It("should trigger multiple HotBackups with CronHotBackup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-5")

		By("creating cron hot backup")
		hbSpec := &hazelcastv1alpha1.HotBackupSpec{}
		chb := hazelcastconfig.CronHotBackup(hzLookupKey, "*/5 * * * * *", hbSpec, labels)
		Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())

		By("waiting cron hot backup two crete two backups")
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

	DescribeTable("Should successfully restore from external backup using HotBackupResourceName", func(bucketURI, secretName string) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-6")
		clusterSize := int32(3)

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, labels)
		hazelcast.Spec.ClusterSize = &clusterSize

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("triggering backup")
		hotBackup := hazelcastconfig.HotBackupAgent(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		By("adding new entries after backup")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 15)

		RemoveHazelcastCR(hazelcast)

		By("creating cluster from backup")
		hazelcast = hazelcastconfig.HazelcastRestore(hzLookupKey, &hazelcastv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}, labels)
		hazelcast.Spec.ClusterSize = &clusterSize
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		assertHazelcastRestoreStatus(hazelcast, hazelcastv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 10, 1*Minute)

	},
		Entry("using GCP bucket", Label("slow"), "gs://operator-e2e-external-backup", "br-secret-gcp"),
	)
})

func unixMilli(msec int64) Time {
	return Unix(msec/1e3, (msec%1e3)*1e6)
}
