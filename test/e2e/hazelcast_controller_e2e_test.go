package e2e

import (
	"bufio"
	"context"
	"fmt"
	"time"

	hzTypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
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
			context.Background(), &hazelcastcomv1alpha1.Map{}, client.InNamespace(hzNamespace))).Should(Succeed())
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
		It("should create Hazelcast cluster", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
		})
	})

	Describe("Hazelcast CR with expose externally feature", func() {
		ctx := context.Background()
		assertExternalAddressesNotEmpty := func() {
			By("status external addresses should not be empty")
			Eventually(func() string {
				hz := &hazelcastcomv1alpha1.Hazelcast{}
				err := k8sClient.Get(context.Background(), lookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.ExternalAddresses
			}, timeout, interval).Should(Not(BeEmpty()))
		}

		It("should create Hazelcast cluster and allow connecting with Hazelcast unisocket client", Label("slow"), func() {
			assertUseHazelcastUnisocket := func() {
				FillTheMapData(ctx, false, "map", 100)
			}
			hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
			assertUseHazelcastUnisocket()
			assertExternalAddressesNotEmpty()
		})

		It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", Label("slow"), func() {
			assertUseHazelcastSmart := func() {
				FillTheMapData(ctx, false, "map", 100)
			}
			hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
			assertUseHazelcastSmart()
			assertExternalAddressesNotEmpty()
		})

		It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", Label("slow"), func() {
			assertUseHazelcastSmart := func() {
				FillTheMapData(ctx, false, "map", 100)
			}
			hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
			assertUseHazelcastSmart()
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", Label("fast"), func() {
			hazelcast := hazelcastconfig.ClusterName(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		})
	})

	Context("Hazelcast member status", func() {

		It("should update HZ ready members status", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(lookupKey, 3)
			assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				err := k8sClient.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace(lookupKey.Namespace), client.MatchingLabels{
					n.ApplicationNameLabel:         n.Hazelcast,
					n.ApplicationInstanceNameLabel: hazelcast.Name,
					n.ApplicationManagedByLabel:    n.OperatorName,
				})
				Expect(err).ToNot(HaveOccurred())
				evaluateReadyMembers(lookupKey, 3)
			})
		})

		It("should update HZ detailed member status", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)
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

		It("should be reflected to Hazelcast CR status", Label("fast"), func() {
			createWithoutCheck(hazelcastconfig.Faulty(hzNamespace, ee))
			assertStatusAndMessageEventually(hazelcastcomv1alpha1.Failed)
		})
	})

	Describe("Hazelcast CR with Persistence feature enabled", func() {
		It("should enable persistence for members successfully", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, "/data/hot-restart")
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

		It("should successfully trigger HotBackup", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, "/data/hot-restart")
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			By("Check the HotBackup creation sequence")
			logs := InitLogs(t)
			defer logs.Close()
			scanner := bufio.NewScanner(logs)
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Starting new hot backup with sequence"))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(MatchRegexp("Backup of hot restart store \\S+ finished"))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"))
			Expect(logs.Close()).Should(Succeed())

			hb := &hazelcastcomv1alpha1.HotBackup{}
			Eventually(func() hazelcastcomv1alpha1.HotBackupState {
				err := k8sClient.Get(
					context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
				Expect(err).ToNot(HaveOccurred())
				return hb.Status.State
			}, timeout, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupSuccess))
		})

		It("should trigger ForceStart when restart from HotBackup failed", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, "/data/hot-restart", false)
			CreateHazelcastCR(hazelcast)
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
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(lookupKey, 2)
		})

		DescribeTable("should successfully restart from HotBackup data", func(params ...interface{}) {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			baseDir := "/data/hot-restart"
			hazelcast := addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, params...), getFirstWorkerNodeName())
			CreateHazelcastCR(hazelcast)
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
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Starting hot-restart service. Base directory: " + baseDir))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Starting the Hot Restart procedure."))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Local Hot Restart procedure completed with success."))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Completed hot restart with final cluster state: ACTIVE"))
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(MatchRegexp("Hot Restart procedure completed in \\d+ seconds"))

			Expect(logs.Close()).Should(Succeed())

			Eventually(func() *hazelcastcomv1alpha1.RestoreStatus {
				hz := &hazelcastcomv1alpha1.Hazelcast{}
				_ = k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      hzName,
					Namespace: hzNamespace,
				}, hz)
				return hz.Status.Restore
			}, timeout, interval).Should(And(
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
	Describe("Hazelcast Map Config", func() {
		It("should create Map Config", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-1", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		})

		It("should create Map Config with correct default values", Label("fast"), func() {
			localPort := "8000"
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			By("port-forwarding to Hazelcast master pod")
			stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)
			err := waitForReadyChannel(readyChan, 5*time.Second)
			Expect(err).To(BeNil())

			By("creating the map config successfully")
			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-123", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map config is created correctly")
			cl := createHazelcastClient(context.Background(), hazelcast, localPort)
			defer func() {
				err := cl.Shutdown(context.Background())
				Expect(err).To(BeNil())
			}()
			mapConfig := getMapConfig(context.Background(), cl, m.MapName())
			Expect(mapConfig.InMemoryFormat).Should(Equal(int32(0)))
			Expect(mapConfig.BackupCount).Should(Equal(n.DefaultMapBackupCount))
			Expect(mapConfig.AsyncBackupCount).Should(Equal(int32(0)))
			Expect(mapConfig.TimeToLiveSeconds).Should(Equal(*m.Spec.TimeToLiveSeconds))
			Expect(mapConfig.MaxIdleSeconds).Should(Equal(*m.Spec.MaxIdleSeconds))
			Expect(mapConfig.MaxSize).Should(Equal(*m.Spec.Eviction.MaxSize))
			Expect(mapConfig.MaxSizePolicy).Should(Equal(hazelcastcomv1alpha1.EncodeMaxSizePolicy[m.Spec.Eviction.MaxSizePolicy]))
			Expect(mapConfig.ReadBackupData).Should(Equal(false))
			Expect(mapConfig.EvictionPolicy).Should(Equal(hazelcastcomv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy]))
			Expect(mapConfig.MergePolicy).Should(Equal("com.hazelcast.spi.merge.PutIfAbsentMergePolicy"))

		})
		It("should create Map Config with Indexes", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map", hzNamespace)
			m.Spec.BackupCount = pointer.Int32Ptr(3)
			m.Spec.Indexes = []hazelcastcomv1alpha1.IndexConfig{
				{
					Name:               "index-1",
					Type:               hazelcastcomv1alpha1.IndexTypeHash,
					Attributes:         []string{"attribute1", "attribute2"},
					BitmapIndexOptions: nil,
				},
				{
					Name:       "index-2",
					Type:       hazelcastcomv1alpha1.IndexTypeBitmap,
					Attributes: []string{"attribute3", "attribute4"},
					BitmapIndexOptions: &hazelcastcomv1alpha1.BitmapIndexOptionsConfig{
						UniqueKey:           "key",
						UniqueKeyTransition: hazelcastcomv1alpha1.UniqueKeyTransitionRAW,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			localPort := "8000"
			By("port-forwarding to Hazelcast master pod")
			stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)
			err := waitForReadyChannel(readyChan, 5*time.Second)
			Expect(err).To(BeNil())

			cl := createHazelcastClient(context.Background(), hazelcast, localPort)
			defer func() {
				err := cl.Shutdown(context.Background())
				Expect(err).To(BeNil())
			}()

			By("checking if the map config is created correctly")
			mapConfig := getMapConfig(context.Background(), cl, m.MapName())
			Expect(mapConfig.Indexes[0].Name).Should(Equal("index-1"))
			Expect(mapConfig.Indexes[0].Type).Should(Equal(hazelcastcomv1alpha1.EncodeIndexType[hazelcastcomv1alpha1.IndexTypeHash]))
			Expect(mapConfig.Indexes[0].Attributes).Should(Equal([]string{"attribute1", "attribute2"}))
			// TODO: Hazelcast side returns these bitmapIndexOptions even though we give them empty.
			Expect(mapConfig.Indexes[0].BitmapIndexOptions.UniqueKey).Should(Equal("__key"))
			Expect(mapConfig.Indexes[0].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(int32(0)))

			Expect(mapConfig.Indexes[1].Name).Should(Equal("index-2"))
			Expect(mapConfig.Indexes[1].Type).Should(Equal(hazelcastcomv1alpha1.EncodeIndexType[hazelcastcomv1alpha1.IndexTypeBitmap]))
			Expect(mapConfig.Indexes[1].Attributes).Should(Equal([]string{"attribute3", "attribute4"}))
			Expect(mapConfig.Indexes[1].BitmapIndexOptions.UniqueKey).Should(Equal("key"))
			Expect(mapConfig.Indexes[1].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(hazelcastcomv1alpha1.EncodeUniqueKeyTransition[hazelcastcomv1alpha1.UniqueKeyTransitionRAW]))

		})

		It("should fail when persistence of Map CR and Hazelcast CR do not match", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-3", hzNamespace)
			m.Spec.PersistenceEnabled = true

			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
			Expect(m.Status.Message).To(Equal(fmt.Sprintf("Persistence is not enabled for the Hazelcast resource %s.", hazelcast.Name)))
		})

		It("should update the map correctly", Label("fast"), func() {
			localPort := "8000"
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			By("port-forwarding to Hazelcast master pod")
			stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)
			err := waitForReadyChannel(readyChan, 5*time.Second)
			Expect(err).To(BeNil())

			By("creating the map config successfully")
			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-123", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("updating the map config")
			m.Spec.TimeToLiveSeconds = pointer.Int32Ptr(150)
			m.Spec.MaxIdleSeconds = pointer.Int32Ptr(100)
			m.Spec.Eviction = &hazelcastcomv1alpha1.EvictionConfig{
				EvictionPolicy: hazelcastcomv1alpha1.EvictionPolicyLFU,
				MaxSize:        pointer.Int32Ptr(500),
				MaxSizePolicy:  hazelcastcomv1alpha1.MaxSizePolicyFreeHeapSize,
			}
			Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map config is updated correctly")
			cl := createHazelcastClient(context.Background(), hazelcast, localPort)
			defer func() {
				err := cl.Shutdown(context.Background())
				Expect(err).To(BeNil())
			}()
			mapConfig := getMapConfig(context.Background(), cl, m.MapName())
			Expect(mapConfig.TimeToLiveSeconds).Should(Equal(*m.Spec.TimeToLiveSeconds))
			Expect(mapConfig.MaxIdleSeconds).Should(Equal(*m.Spec.MaxIdleSeconds))
			Expect(mapConfig.ReadBackupData).Should(Equal(false))
			Expect(mapConfig.MaxSize).Should(Equal(*m.Spec.Eviction.MaxSize))
			Expect(mapConfig.MaxSizePolicy).Should(Equal(hazelcastcomv1alpha1.EncodeMaxSizePolicy[m.Spec.Eviction.MaxSizePolicy]))
			Expect(mapConfig.EvictionPolicy).Should(Equal(hazelcastcomv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy]))
		})

		It("should fail to update", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			By("creating the map config successfully")
			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-123", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("failing to update map config")
			m.Spec.BackupCount = pointer.Int32Ptr(3)
			Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
		})

		It("should keep the entries after a Hot Backup", Label("slow"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			localPort := "8000"
			mapName := "map123"
			baseDir := "/data/hot-restart"

			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(lookupKey, 3)

			By("port-forwarding to Hazelcast master pod")
			stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			err := waitForReadyChannel(readyChan, 5*time.Second)
			Expect(err).To(BeNil())

			By("creating the map config successfully")
			m := hazelcastconfig.DefaultMap(hazelcast.Name, mapName, hzNamespace)
			m.Spec.PersistenceEnabled = true
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("Filling the map with entries")
			entryCount := 100
			cl := createHazelcastClient(context.Background(), hazelcast, localPort)
			mp, err := cl.GetMap(context.Background(), mapName)
			Expect(err).To(BeNil())

			entries := make([]hzTypes.Entry, entryCount)
			for i := 0; i < entryCount; i++ {
				entries[i] = hzTypes.NewEntry(i, "val")
			}
			err = mp.PutAll(context.Background(), entries...)
			Expect(err).To(BeNil())
			Expect(mp.Size(context.Background())).Should(Equal(entryCount))

			By("Shutting down the connection to cluster")
			err = cl.Shutdown(context.Background())
			Expect(err).To(BeNil())
			closeChannel(stopChan)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			seq := GetBackupSequence(t)
			RemoveHazelcastCR(hazelcast)

			By("Creating new Hazelcast cluster from existing backup")
			baseDir += "/hot-backup/backup-" + seq
			hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir)

			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
			evaluateReadyMembers(lookupKey, 3)
			assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)

			By("port-forwarding to restarted Hazelcast master pod")
			stopChan, readyChan = portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)
			err = waitForReadyChannel(readyChan, 5*time.Second)
			Expect(err).To(BeNil())

			By("Checking the map entries")
			cl = createHazelcastClient(context.Background(), hazelcast, localPort)
			defer func() {
				err := cl.Shutdown(context.Background())
				Expect(err).To(BeNil())
			}()
			mp, err = cl.GetMap(context.Background(), mapName)
			Expect(err).To(BeNil())
			Expect(mp.Size(context.Background())).Should(Equal(entryCount))
		})

		It("should persist the map successfully created configs into the configmap", Label("fast"), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			maps := []string{"map1", "map2", "map3", "mapfail"}

			hazelcast := hazelcastconfig.Default(hzNamespace, true)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(lookupKey, 3)

			By("creating the map configs successfully")
			for i, mapp := range maps {
				m := hazelcastconfig.DefaultMap(hazelcast.Name, mapp, hzNamespace)
				m.Spec.Eviction = &hazelcastcomv1alpha1.EvictionConfig{MaxSize: pointer.Int32Ptr(int32(i) * 100)}
				m.Spec.HazelcastResourceName = hazelcast.Name
				if mapp == "mapfail" {
					m.Spec.HazelcastResourceName = "failedHz"
				}
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				if mapp == "mapfail" {
					assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
					continue
				}
				assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
			}

			By("checking if the maps are in the map config")
			hzConfig := assertMapConfigsPersisted(hazelcast, "map1", "map2", "map3")
			for i, mapp := range maps {
				if mapp != "mapfail" {
					Expect(hzConfig.Hazelcast.Map[mapp].Eviction.Size).Should(Equal(int32(i) * 100))
				}
			}

			By("deleting map2")
			Expect(k8sClient.Delete(context.Background(),
				&hazelcastcomv1alpha1.Map{ObjectMeta: v1.ObjectMeta{Name: "map2", Namespace: hazelcast.Namespace}})).Should(Succeed())

			By("checking if map2 is not persisted in the configmap")
			_ = assertMapConfigsPersisted(hazelcast, "map1", "map3")
		})

		It("should persist Map Config with Indexes", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-2", hzNamespace)
			m.Spec.Indexes = []hazelcastcomv1alpha1.IndexConfig{
				{
					Name:       "index-1",
					Type:       hazelcastcomv1alpha1.IndexTypeHash,
					Attributes: []string{"attribute1", "attribute2"},
					BitmapIndexOptions: &hazelcastcomv1alpha1.BitmapIndexOptionsConfig{
						UniqueKey:           "key",
						UniqueKeyTransition: hazelcastcomv1alpha1.UniqueKeyTransitionRAW,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map is in the configmap")
			hzConfig := assertMapConfigsPersisted(hazelcast, "map-2")

			By("checking if the indexes are persisted")
			Expect(hzConfig.Hazelcast.Map["map-2"].Indexes[0].Name).Should(Equal("index-1"))
			Expect(hzConfig.Hazelcast.Map["map-2"].Indexes[0].Type).Should(Equal(string(hazelcastcomv1alpha1.IndexTypeHash)))
			Expect(hzConfig.Hazelcast.Map["map-2"].Indexes[0].Attributes).Should(ConsistOf("attribute1", "attribute2"))
			Expect(hzConfig.Hazelcast.Map["map-2"].Indexes[0].BitmapIndexOptions.UniqueKey).Should(Equal("key"))
			Expect(hzConfig.Hazelcast.Map["map-2"].Indexes[0].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(string(hazelcastcomv1alpha1.UniqueKeyTransitionRAW)))
		})

		It("should continue persisting last applied Map Config in case of failure", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-2", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map config is persisted")
			hzConfig := assertMapConfigsPersisted(hazelcast, "map-2")
			mcfg := hzConfig.Hazelcast.Map["map-2"]

			By("failing to update the map config")
			m.Spec.BackupCount = pointer.Int32Ptr(4)
			Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)

			By("checking if the same map config is still there")
			// Should wait for Hazelcast reconciler to get triggered, we do not have a waiting mechanism for that.
			time.Sleep(5 * time.Second)
			hzConfig = assertMapConfigsPersisted(hazelcast, "map-2")
			newMcfg := hzConfig.Hazelcast.Map["map-2"]
			Expect(newMcfg).To(Equal(mcfg))

		})
	})
})
