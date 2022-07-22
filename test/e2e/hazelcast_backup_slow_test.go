package e2e

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Backup", Label("backup_slow"), func() {
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
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	It("should restart successfully after shutting down Hazelcast", Label("slow"), func() {
		setLabelAndCRName("hbs-1")
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapData(ctx, hzLookupKey, true, m.Name, 100)

		By("creating new Hazelcast cluster")
		RemoveHazelcastCR(hazelcast)
		t := Now()
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		waitForMapSize(context.Background(), hzLookupKey, m.Name, 100)
	})

	It("should successfully start after one member restart", Label("slow"), func() {
		setLabelAndCRName("hbs-2")
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		t := Now()
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapData(ctx, hzLookupKey, true, m.Name, 100)

		By("deleting the pod")
		DeletePod(hazelcast.Name+"-2", 0)
		evaluateReadyMembers(hzLookupKey, 3)

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		waitForMapSize(context.Background(), hzLookupKey, m.Name, 100)
	})

	It("should restore 9 GB data after planned shutdown", Label("slow"), func() {
		setLabelAndCRName("hbs-3")
		var mapSizeInGb = 3
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")}[0]
		CreateHazelcastCR(hazelcast)

		By("creating the map config")
		dm := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		dm.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapWithHugeData(ctx, dm.Name, mapSizeInGb, hazelcast)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		seq := GetBackupSequence(t, hzLookupKey)

		By("Check the HotBackup creation sequence")
		hb := &hazelcastcomv1alpha1.HotBackup{}
		Eventually(func() hazelcastcomv1alpha1.HotBackupState {
			err := k8sClient.Get(
				context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
			Expect(err).ToNot(HaveOccurred())
			return hb.Status.State
		}, 10*Minute, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupSuccess))

		By("removing Hazelcast CR")
		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from the existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")}[0]

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)

		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Starting hot-restart service. Base directory: " + baseDir))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Starting the Hot Restart procedure."))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Local Hot Restart procedure completed with success."))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())
		assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)

		By("checking the Map size")
		waitForMapSize(context.Background(), hzLookupKey, dm.Name, int(float64(mapSizeInGb)*math.Round(1310.72)*100))
	})

	It("Should successfully restore 9 Gb data from external backup using GCP bucket", Label("slow"), func() {
		setLabelAndCRName("hbs-4")
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		ctx := context.Background()
		var mapSizeInGb = 3
		var bucketURI = "gs://operator-e2e-external-backup"
		var secretName = "br-secret-gcp"

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.ExternalBackup(hzLookupKey, true, labels)
		hazelcast.Spec.ClusterSize = &[]int32{3}[0]
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")}[0]

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("creating the map config")
		dm := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		dm.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapWithHugeData(ctx, dm.Name, mapSizeInGb, hazelcast)

		By("triggering the backup")
		t := Now()
		hotBackup := hazelcastconfig.HotBackupAgent(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		By("waiting for backup to finish")
		hb := &hazelcastcomv1alpha1.HotBackup{}
		Eventually(func() hazelcastcomv1alpha1.HotBackupState {
			err := k8sClient.Get(
				context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
			Expect(err).ToNot(HaveOccurred())
			Expect(hb.Status.State).ShouldNot(Equal(hazelcastcomv1alpha1.HotBackupFailure), "Message: %v", hb.Status.Message)
			return hb.Status.State
		}, 20*Minute, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupSuccess))

		seq := GetBackupSequence(t, hzLookupKey)

		By("removing cluster")
		RemoveHazelcastCR(hazelcast)

		timestamp, _ := strconv.ParseInt(seq, 10, 64)
		bucketURI += fmt.Sprintf("?prefix=%s/%s/", hzLookupKey.Name,
			unixMilli(timestamp).UTC().Format("2006-01-02-15-04-05"))

		By("creating cluster from external backup")
		hazelcast = hazelcastconfig.ExternalRestore(hzLookupKey, true, labels, bucketURI, secretName)
		hazelcast.Spec.ClusterSize = &[]int32{3}[0]
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(mapSizeInGb) + "Gi")}[0]

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Found existing hot-restart directory"))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Local Hot Restart procedure completed with success."))

		By("checking the Map size")
		waitForMapSize(context.Background(), hzLookupKey, dm.Name, int(float64(mapSizeInGb)*math.Round(1310.72)*100))
	})
})
