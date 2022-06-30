package e2e

import (
	"bufio"
	"context"
	"math"
	"strconv"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
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
		client := GetHzClient(ctx, hzLookupKey, true)
		defer func() {
			err := client.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()
		cl, err := client.GetMap(ctx, m.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
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
		client := GetHzClient(ctx, hzLookupKey, true)
		defer func() {
			err := client.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()
		cl, err := client.GetMap(ctx, m.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
	})

	It("should restore 10 GB data after planned shutdown", Label("slow"), func() {
		setLabelAndCRName("hbs-3")
		var mapSizeInGb = "10"
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
				corev1.ResourceMemory: resource.MustParse(mapSizeInGb + "Gi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(mapSizeInGb + "Gi")}[0]
		CreateHazelcastCR(hazelcast)

		By("creating the map config successfully")
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

		By("creating new Hazelcast cluster from existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(mapSizeInGb + "Gi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(mapSizeInGb + "Gi")}[0]

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
		var m *hzClient.Map
		mapSize, _ := strconv.ParseFloat(mapSizeInGb, 64)
		client := GetHzClient(ctx, hzLookupKey, true)
		defer func() {
			err := client.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()
		m, _ = client.GetMap(ctx, dm.Name)
		// 1310.72 entries per one Go routine.  Formula: 1073741824 Bytes per 1Gb  / 8192 Bytes per entry / 100 go routines
		Eventually(func() (int, error) {
			return m.Size(ctx)
		}, 20*Minute, interval).Should(Equal(int(math.Round(mapSize*1310.72) * 100)))
	})
})
