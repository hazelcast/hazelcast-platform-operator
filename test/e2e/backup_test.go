package e2e

import (
	"bufio"
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hzClient "github.com/hazelcast/hazelcast-go-client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Backup", func() {

	var lookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}

	var controllerManagerName = types.NamespacedName{
		Name:      controllerManagerName(),
		Namespace: hzNamespace,
	}

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
			}, 10*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			context.Background(), &hazelcastcomv1alpha1.HotBackup{}, client.InNamespace(hzNamespace))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			context.Background(), &hazelcastcomv1alpha1.Map{}, client.InNamespace(hzNamespace))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			context.Background(), &corev1.PersistentVolumeClaim{}, client.InNamespace(hzNamespace))).Should(Succeed())
		assertDoesNotExist(lookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	It("should restart successfully after shutting down Hazelcast", Label("slow", "backup"), func() {
		mapName := "backup-map-1"
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(hazelcast.Name, mapName, hzNamespace)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapData(ctx, true, mapName, 100)

		By("creating new Hazelcast cluster")
		RemoveHazelcastCR(hazelcast)
		t := Now()
		hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		logs := InitLogs(t)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		client := GetHzClient(ctx, true)
		defer func() {
			err := client.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()
		cl, err := client.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
	})

	It("should successfully start after one member restart", Label("slow", "backup"), func() {
		mapName := "backup-map-2"
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		t := Now()
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(hazelcast.Name, mapName, hzNamespace)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapData(ctx, true, mapName, 100)

		By("deleting the pod")
		DeletePod(hzName+"-2", 0)
		evaluateReadyMembers(lookupKey, 3)

		logs := InitLogs(t)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		client := GetHzClient(ctx, true)
		defer func() {
			err := client.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()
		cl, err := client.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
	})

	It("should restore 2 GB data after planned shutdown", Label("slow", "backup"), func() {
		var mapSizeInGb = "2"
		var mapName = "backup-map-3"
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(mapSizeInGb + "Gi")},
		}
		CreateHazelcastCR(hazelcast)

		By("creating the map config successfully")
		dm := hazelcastconfig.DefaultMap(hazelcast.Name, mapName, hzNamespace)
		dm.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapWithHugeData(ctx, mapName, mapSizeInGb, hazelcast)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		seq := GetBackupSequence(t)

		By("removing Hazelcast CR")
		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(mapSizeInGb + "Gi")},
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		logs := InitLogs(t)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)

		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Starting hot-restart service. Base directory: " + baseDir))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Starting the Hot Restart procedure."))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Local Hot Restart procedure completed with success."))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		var m *hzClient.Map
		mapSize, _ := strconv.ParseFloat(mapSizeInGb, 64)
		client := GetHzClient(ctx, true)
		defer func() {
			err := client.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()
		m, _ = client.GetMap(ctx, mapName)
		// 1310.72 entries per one Go routine.  Formula: 1073741824 Bytes per 1Gb  / 8192 Bytes per entry / 100 go routines
		Eventually(func() (int, error) {
			return m.Size(ctx)
		}, 5*Minute, interval).Should(Equal(int(math.Round(mapSize*1310.72) * 100)))
	})
})
