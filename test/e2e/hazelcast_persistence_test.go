package e2e

import (
	"bufio"
	"context"
	"fmt"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Persistence feature enabled", Label("hz_persistence"), func() {
	hzName := fmt.Sprintf("hz-persistence-%d", GinkgoParallelProcess())
	hbName := fmt.Sprintf("hz-hb-%d", GinkgoParallelProcess())
	mapName := fmt.Sprintf("hz-map-%d", GinkgoParallelProcess())

	var hzLookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}
	var mapLookupKey = types.NamespacedName{
		Name:      mapName,
		Namespace: hzNamespace,
	}

	var hbLookupKey = types.NamespacedName{
		Name:      hbName,
		Namespace: hzNamespace,
	}
	labels := map[string]string{
		"test_suite": fmt.Sprintf("hz_persistence-%d", GinkgoParallelProcess()),
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
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(hzLookupKey), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			context.Background(), &hazelcastcomv1alpha1.HotBackup{}, client.InNamespace(hzNamespace), client.MatchingLabels(labels))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			context.Background(), &hazelcastcomv1alpha1.Map{}, client.InNamespace(hzNamespace), client.MatchingLabels(labels))).Should(Succeed())
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	It("should enable persistence for members successfully", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
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
		ctx := context.Background()
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels)
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
		Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("filling the Map")
		FillTheMapData(ctx, hzLookupKey, true, mapName, 100)

		By("Creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		By("Check the HotBackup creation sequence")
		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(ContainSubstring("Starting new hot backup with sequence"))
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(MatchRegexp("Backup of hot restart store \\S+ finished"))
		test.EventuallyInLogs(scanner, 15*Second, logInterval).
			Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"))
		Expect(logs.Close()).Should(Succeed())

		hb := &hazelcastcomv1alpha1.HotBackup{}
		Eventually(func() hazelcastcomv1alpha1.HotBackupState {
			err := k8sClient.Get(
				context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
			Expect(err).ToNot(HaveOccurred())
			return hb.Status.State
		}, 10*Minute, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupSuccess))

		By("checking the Map size")
		client := GetHzClient(ctx, hzLookupKey, true)
		defer func() {
			err := client.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()
		cl, err := client.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Size(ctx)).Should(BeEquivalentTo(100))
	})

	It("should trigger ForceStart when restart from HotBackup failed", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-restart", labels, false)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("Creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		seq := GetBackupSequence(t, hzLookupKey)
		RemoveHazelcastCR(hazelcast)

		By("Creating new Hazelcast cluster from existing backup with 2 members")
		baseDir := "/data/hot-restart/hot-backup/backup-" + seq
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, false)
		hazelcast.Spec.ClusterSize = &[]int32{2}[0]
		hazelcast.Spec.Persistence.DataRecoveryTimeout = 60
		hazelcast.Spec.Persistence.AutoForceStart = true
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 2)
	})

	DescribeTable("should successfully restart from HotBackup data", func(params ...interface{}) {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		baseDir := "/data/hot-restart"
		hazelcast := addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, params...), getFirstWorkerNodeName())
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("Creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		seq := GetBackupSequence(t, hzLookupKey)
		RemoveHazelcastCR(hazelcast)

		By("Creating new Hazelcast cluster from existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels, params...), getFirstWorkerNodeName())

		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey, 3)

		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()

		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).
			Should(ContainSubstring("Starting hot-restart service. Base directory: " + baseDir))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).
			Should(ContainSubstring("Starting the Hot Restart procedure."))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).
			Should(ContainSubstring("Local Hot Restart procedure completed with success."))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).
			Should(ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).
			Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))

		Expect(logs.Close()).Should(Succeed())

		Eventually(func() *hazelcastcomv1alpha1.RestoreStatus {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			_ = k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      hzName,
				Namespace: hzNamespace,
			}, hz)
			return hz.Status.Restore
		}, 5*Second, interval).Should(And(
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
})
