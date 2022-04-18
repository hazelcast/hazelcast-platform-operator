package e2e

import (
	"bufio"
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

const (
	hzName      = "hazelcast"
	logInterval = 10 * time.Millisecond
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
			context.Background(), &corev1.PersistentVolumeClaim{}, client.InNamespace(hzNamespace))).Should(Succeed())
		assertDoesNotExist(lookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	createWithoutCheck := func(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
		By("Creating Hazelcast CR", func() {
			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		})
	}

	Describe("Default Hazelcast CR", func() {
		It("should create Hazelcast cluster", func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast, lookupKey)
		})
	})

	Describe("Hazelcast CR with expose externally feature", func() {
		ctx := context.Background()
		assertExternalAddressesNotEmpty := func() {
			By("status external addresses should not be empty")
			Eventually(func() string {
				cr := hazelcastcomv1alpha1.Hazelcast{}
				err := k8sClient.Get(context.Background(), lookupKey, &cr)
				Expect(err).ToNot(HaveOccurred())
				return cr.Status.ExternalAddresses
			}, timeout, interval).Should(Not(BeEmpty()))
		}

		It("should create Hazelcast cluster and allow connecting with Hazelcast unisocket client", func() {
			assertUseHazelcastUnisocket := func() {
				FillTheMapData(ctx, true, "map", 100)
			}
			hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzNamespace, ee)
			CreateHazelcastCR(hazelcast, lookupKey)
			assertUseHazelcastUnisocket()
			assertExternalAddressesNotEmpty()
		})

		It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", func() {
			assertUseHazelcastSmart := func() {
				FillTheMapData(ctx, false, "map", 100)
			}

			hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzNamespace, ee)
			CreateHazelcastCR(hazelcast, lookupKey)
			assertUseHazelcastSmart()
			assertExternalAddressesNotEmpty()
		})

		It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", func() {
			assertUseHazelcastSmart := func() {
				FillTheMapData(ctx, false, "map", 100)
			}
			hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzNamespace, ee)
			CreateHazelcastCR(hazelcast, lookupKey)
			assertUseHazelcastSmart()
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", func() {
			hazelcast := hazelcastconfig.ClusterName(hzNamespace, ee)
			CreateHazelcastCR(hazelcast, lookupKey)
			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		})
	})

	Context("Hazelcast member status", func() {

		It("should update HZ ready members status", func() {
			h := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(h, lookupKey)
			evaluateReadyMembers(lookupKey, 3)

			assertMemberLogs(h, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				err := k8sClient.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace(lookupKey.Namespace), client.MatchingLabels{
					n.ApplicationNameLabel:         n.Hazelcast,
					n.ApplicationInstanceNameLabel: h.Name,
					n.ApplicationManagedByLabel:    n.OperatorName,
				})
				Expect(err).ToNot(HaveOccurred())
				evaluateReadyMembers(lookupKey, 3)
			})
		})

		It("should update HZ detailed member status", func() {
			h := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(h, lookupKey)
			evaluateReadyMembers(lookupKey, 3)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			memberStateT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) string {
				return status.State
			}
			masterT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) bool {
				return status.Master
			}
			Eventually(func() []hazelcastcomv1alpha1.HazelcastMemberStatus {
				err := k8sClient.Get(context.Background(), lookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Members
			}, timeout, interval).Should(And(HaveLen(3),
				ContainElement(WithTransform(memberStateT, Equal("ACTIVE"))),
				ContainElement(WithTransform(masterT, Equal(true))),
			))
		})
	})

	Describe("External API errors", func() {
		assertStatusAndMessageEventually := func(phase hazelcastcomv1alpha1.Phase) {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), lookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Phase
			}, timeout, interval).Should(Equal(phase))
			Expect(hz.Status.Message).Should(Not(BeEmpty()))
		}

		It("should be reflected to Hazelcast CR status", func() {
			createWithoutCheck(hazelcastconfig.Faulty(hzNamespace, ee))
			assertStatusAndMessageEventually(hazelcastcomv1alpha1.Failed)
		})
	})

	Describe("Hazelcast CR with Persistence feature enabled", func() {
		It("should enable persistence for members successfully", func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, "/data/hot-restart")
			CreateHazelcastCR(hazelcast, lookupKey)
			assertMemberLogs(hazelcast, "Local Hot Restart procedure completed with success.")
			assertMemberLogs(hazelcast, "Hot Restart procedure completed")
			assertPersistenceVolumeExist(hazelcast)
		})

		It("should successfully trigger HotBackup", func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, "/data/hot-restart")
			CreateHazelcastCR(hazelcast, lookupKey)
			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			By("Check the HotBackup creation sequence")
			logs := InitLogs(t)
			defer logs.Close()
			scanner := bufio.NewScanner(logs)
			ReadLogs(scanner, ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
			ReadLogs(scanner, ContainSubstring("Starting new hot backup with sequence"))
			ReadLogs(scanner, ContainSubstring("Backup of hot restart store \\S+ finished"))
			ReadLogs(scanner, ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"))
			Expect(logs.Close()).Should(Succeed())

			hb := &hazelcastcomv1alpha1.HotBackup{}
			Eventually(func() hazelcastcomv1alpha1.HotBackupState {
				err := k8sClient.Get(
					context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
				Expect(err).ToNot(HaveOccurred())
				return hb.Status.State
			}, timeout, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupSuccess))
		})

		It("should trigger ForceStart when restart from HotBackup failed", func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, "/data/hot-restart", false)
			CreateHazelcastCR(hazelcast, lookupKey)
			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			seq := GetBackupSequence(t)
			RemoveHazelcastCR(hazelcast)

			By("Creating new Hazelcast cluster from existing backup with 2 members")
			baseDir := "/data/hot-restart/hot-backup/backup-" + seq
			hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
			hazelcast.Spec.ClusterSize = &[]int32{2}[0]
			hazelcast.Spec.Persistence.DataRecoveryTimeout = 60
			hazelcast.Spec.Persistence.AutoForceStart = true
			CreateHazelcastCR(hazelcast, lookupKey)
			evaluateReadyMembers(lookupKey, 2)
		})

		DescribeTable("should successfully restart from HotBackup data", func(params ...interface{}) {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			baseDir := "/data/hot-restart"
			hazelcast := addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, params...), getFirstWorkerNodeName())
			CreateHazelcastCR(hazelcast, lookupKey)
			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			seq := GetBackupSequence(t)
			RemoveHazelcastCR(hazelcast)

			By("Creating new Hazelcast cluster from existing backup")
			baseDir += "/hot-backup/backup-" + seq
			hazelcast = addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, params...), getFirstWorkerNodeName())

			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
			evaluateReadyMembers(lookupKey, 3)

			logs := InitLogs(t)
			defer logs.Close()
			scanner := bufio.NewScanner(logs)
			ReadLogs(scanner, ContainSubstring("Starting hot-restart service. Base directory: "+baseDir))
			ReadLogs(scanner, ContainSubstring("Starting the Hot Restart procedure."))
			ReadLogs(scanner, ContainSubstring("Local Hot Restart procedure completed with success."))
			ReadLogs(scanner, ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
			ReadLogs(scanner, MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))
			Expect(logs.Close()).Should(Succeed())
		},
			Entry("with PVC configuration"),
			Entry("with HostPath configuration single node", "/tmp/hazelcast/singleNode", "dummyNodeName"),
			Entry("with HostPath configuration multiple nodes", "/tmp/hazelcast/multiNode"),
		)
	})
})
