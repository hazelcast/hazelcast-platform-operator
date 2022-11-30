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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Persistence feature enabled", Label("hz_persistence"), func() {
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
		DeleteAllOf(&hazelcastcomv1alpha1.CronHotBackup{}, &hazelcastcomv1alpha1.CronHotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, &hazelcastcomv1alpha1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())

	})

	It("should enable persistence for members successfully", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-1")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels)
		CreateHazelcastCR(hazelcast)
		assertMemberLogs(hazelcast, "Local Hot Restart procedure completed with success.")
		assertMemberLogs(hazelcast, "Hot Restart procedure completed")

		pods := &corev1.PodList{}
		podLabels := client.MatchingLabels{
			n.ApplicationNameLabel:         n.Hazelcast,
			n.ApplicationInstanceNameLabel: hazelcast.Name,
			n.ApplicationManagedByLabel:    n.OperatorName,
		}
		if err := k8sClient.List(context.Background(), pods, client.InNamespace(hazelcast.Namespace), podLabels); err != nil {
			Fail("Could not find Pods for Hazelcast " + hazelcast.Name)
		}

		for _, pod := range pods.Items {
			Expect(pod.Spec.Volumes).Should(ContainElement(corev1.Volume{
				Name: n.PersistenceVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: n.PersistenceVolumeName + "-" + pod.Name,
					},
				},
			}))
		}
	})

	It("should successfully trigger HotBackup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		ctx := context.Background()
		setLabelAndCRName("hp-2")

		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("creating the map config")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		FillTheMapData(ctx, hzLookupKey, true, m.Name, 100)

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
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(ContainSubstring("Starting new hot backup with sequence"))
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"))
		Expect(logs.Close()).Should(Succeed())

		assertHotBackupSuccess(hotBackup, 1*Minute)

		WaitForMapSize(ctx, hzLookupKey, m.Name, 100, 1*Minute)
	})

	It("should trigger ForceStart when restart from HotBackup failed", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-3")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels, false)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

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
		evaluateReadyMembers(hzLookupKey, 2)
	})

	FDescribeTable("should successfully restart from HotBackup data", func(params ...interface{}) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-4")
		baseDir := "/data/hot-restart"
		hazelcast := addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, params...), getFirstWorkerNodeName())
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		seq := GetBackupSequence(t, hzLookupKey)
		backupSeqFolder := hotBackup.Status.GetBackupFolder()
		Expect("backup-" + seq).Should(Equal(backupSeqFolder))
		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, params...), getFirstWorkerNodeName())

		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey, 3)

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()

		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 30*Second, logInterval).
			Should(ContainSubstring("Starting hot-restart service. Base directory: " + baseDir))
		test.EventuallyInLogs(scanner, 30*Second, logInterval).
			Should(ContainSubstring("Starting the Hot Restart procedure."))
		test.EventuallyInLogs(scanner, 30*Second, logInterval).
			Should(ContainSubstring("Local Hot Restart procedure completed with success."))
		test.EventuallyInLogs(scanner, 30*Second, logInterval).
			Should(ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
		test.EventuallyInLogs(scanner, 30*Second, logInterval).
			Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))

		Expect(logs.Close()).Should(Succeed())

		Eventually(func() *hazelcastcomv1alpha1.RestoreStatus {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			_ = k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      hazelcast.Name,
				Namespace: hzNamespace,
			}, hz)
			return hz.Status.Restore
		}, 20*Second, interval).Should(And(
			Not(BeNil()),
			WithTransform(func(h *hazelcastcomv1alpha1.RestoreStatus) hazelcastcomv1alpha1.RestoreState {
				return h.State
			}, Equal(hazelcastcomv1alpha1.RestoreSucceeded)),
		))
	},
		Entry("with PVC configuration", Label("slow")),
		Entry("with HostPath configuration single node", Label("slow"), "/tmp/hazelcast/singleNode", "dummyNodeName"),
		Entry("with HostPath configuration multiple nodes", Label("slow"), "/tmp/hazelcast/multiNode"),
	)

	DescribeTable("Should successfully restore from external backup", func(bucketURI, secretName string) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-5")

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, int(*hazelcast.Spec.ClusterSize))

		By("triggering backup")
		t := Now()
		hotBackup := hazelcastconfig.HotBackupAgent(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)
		backupBucketURI := hotBackup.Status.GetBucketURI()

		seq := GetBackupSequence(t, hzLookupKey)
		RemoveHazelcastCR(hazelcast)
		deletePVCs(hzLookupKey)

		timestamp, _ := strconv.ParseInt(seq, 10, 64)
		bucketURI += fmt.Sprintf("?prefix=%s/%s/", hzLookupKey.Name,
			unixMilli(timestamp).UTC().Format("2006-01-02-15-04-05")) // hazelcast/2022-06-02-21-57-49/
		Expect(bucketURI).Should(Equal(backupBucketURI))

		By("creating cluster from external backup")
		hazelcast = hazelcastconfig.HazelcastRestore(hzLookupKey,
			&hazelcastcomv1alpha1.RestoreConfiguration{
				BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
					BucketURI: bucketURI,
					Secret:    secretName,
				},
			}, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, int(*hazelcast.Spec.ClusterSize))

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Found existing hot-restart directory"))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Local Hot Restart procedure completed with success."))
	},
		Entry("using AWS S3 bucket", Label("slow"), "s3://operator-e2e-external-backup", "br-secret-s3"),
		Entry("using GCP bucket", Label("slow"), "gs://operator-e2e-external-backup", "br-secret-gcp"),
		Entry("using Azure bucket", Label("slow"), "azblob://operator-e2e-external-backup", "br-secret-az"),
	)

	It("should trigger multiple HotBackups with CronHotBackup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-6")

		By("creating cron hot backup")
		hbSpec := &hazelcastcomv1alpha1.HotBackupSpec{}
		chb := hazelcastconfig.CronHotBackup(hzLookupKey, "*/5 * * * * *", hbSpec, labels)
		Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())

		By("waiting cron hot backup two crete two backups")
		Eventually(func() int {
			hbl := &hazelcastcomv1alpha1.HotBackupList{}
			err := k8sClient.List(context.Background(), hbl, client.InNamespace(chb.Namespace), client.MatchingLabels(labels))
			if err != nil {
				return 0
			}
			return len(hbl.Items)
		}, 11*Second, 1*Second).Should(Equal(2))

		By("deleting the cron hot backup")
		DeleteAllOf(&hazelcastcomv1alpha1.CronHotBackup{}, &hazelcastcomv1alpha1.CronHotBackupList{}, hzNamespace, labels)

		By("seeing hot backup CRs are also deleted")
		hbl := &hazelcastcomv1alpha1.HotBackupList{}
		Expect(k8sClient.List(context.Background(), hbl, client.InNamespace(chb.Namespace), client.MatchingLabels(labels))).Should(Succeed())
		Expect(len(hbl.Items)).To(Equal(0))
	})

	DescribeTable("Should successfully restore from HotBackupResourceName", func(bucketURI, secretName string) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hp-7")
		clusterSize := int32(3)

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, labels)
		hazelcast.Spec.ClusterSize = &clusterSize
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type: hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, int(*hazelcast.Spec.ClusterSize))

		By("creating the map config")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		FillTheMapData(context.Background(), hzLookupKey, true, m.Name, 100)

		By("triggering backup")
		hotBackup := hazelcastconfig.HotBackupAgent(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		FillTheMapData(context.Background(), hzLookupKey, true, m.Name, 200)

		RemoveHazelcastCR(hazelcast)

		By("creating cluster from backup")
		hazelcast = hazelcastconfig.HazelcastRestore(hzLookupKey, &hazelcastcomv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}, labels)
		hazelcast.Spec.ClusterSize = &clusterSize
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type: hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, int(*hazelcast.Spec.ClusterSize))
		WaitForMapSize(context.Background(), hzLookupKey, m.Name, 100, 1*Minute)

	},
		Entry("using GCP bucket", Label("slow"), "gs://operator-e2e-external-backup", "br-secret-gcp"),
		Entry("using local backup", Label("slow"), "", ""),
	)
})

func unixMilli(msec int64) Time {
	return Unix(msec/1e3, (msec%1e3)*1e6)
}
