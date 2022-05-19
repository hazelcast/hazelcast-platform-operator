package e2e

import (
	"bufio"
	"context"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"strconv"
	"time"

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

var _ = Describe("Hazelcast", func() {

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
		By("Checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, timeout, interval).Should(Equal(int32(1)))
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

	It("should restart successfully after shutting down Hazelcast", Label("slow"), func() {
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
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(hazelcast.Name, mapName, hzNamespace)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapData(ctx, true, mapName, 100)

		By("creating new Hazelcast cluster")
		RemoveHazelcastCR(hazelcast)
		t := time.Now()
		hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		logs := InitLogs(t)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, timeout, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		client := GetHzClient(ctx, true)
		cl, err := client.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
		client.Shutdown(ctx)
	})

	It("should successfully start after one member restart", Label("slow"), func() {
		mapName := "backup-map-2"
		ctx := context.Background()
		baseDir := "/data/hot-restart"
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		t := time.Now()
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
		test.EventuallyInLogs(scanner, timeout, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
		Expect(logs.Close()).Should(Succeed())

		By("checking the Map size")
		client := GetHzClient(ctx, true)
		cl, err := client.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
		client.Shutdown(ctx)
	})

	It("should restore 1 GB data after planned shutdown", Label("slow"), func() {
		var mapSizeInGb = "1"
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
		CreateHazelcastCR(hazelcast)

		By("creating the map config successfully")
		dm := hazelcastconfig.DefaultMap(hazelcast.Name, mapName, hzNamespace)
		dm.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
		assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapWithHugeData(ctx, mapName, mapSizeInGb, hazelcast)

		By("creating HotBackup CR")
		t := time.Now()
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
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(lookupKey, 3)

		logs := InitLogs(t)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)

		test.EventuallyInLogs(scanner, timeout, logInterval).Should(ContainSubstring("Starting hot-restart service. Base directory: " + baseDir))
		test.EventuallyInLogs(scanner, timeout, logInterval).Should(ContainSubstring("Starting the Hot Restart procedure."))
		test.EventuallyInLogs(scanner, timeout, logInterval).Should(ContainSubstring("Local Hot Restart procedure completed with success."))
		test.EventuallyInLogs(scanner, timeout, logInterval).Should(ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
		test.EventuallyInLogs(scanner, timeout, logInterval).Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
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
		Expect(m.Size(ctx)).Should(Equal(int(math.Round(mapSize*1310.72) * 100)))
		// 1310.72 entries per one Go routine.  Formula: 1073741824 Bytes per 1Gb  / 8192 Bytes per entry / 100 go routines
	})
})
