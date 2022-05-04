package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	hzTypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/platform"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
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

	create := func(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
		By("Creating Hazelcast CR", func() {
			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		})

		By("Checking Hazelcast CR running", func() {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return isHazelcastRunning(hz)
			}, timeout, interval).Should(BeTrue())
		})
	}

	createWithoutCheck := func(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
		By("Creating Hazelcast CR", func() {
			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		})
	}

	Describe("Default Hazelcast CR", func() {
		It("should create Hazelcast cluster", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)
		})
	})

	Describe("Hazelcast CR with expose externally feature", func() {
		assertUseHazelcast := func(unisocket bool) {
			ctx := context.Background()

			By("checking Hazelcast discovery service external IP")
			s := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, s)
				Expect(err).ToNot(HaveOccurred())
				return len(s.Status.LoadBalancer.Ingress) > 0
			}, timeout, interval).Should(BeTrue())

			addr := s.Status.LoadBalancer.Ingress[0].IP
			if addr == "" {
				addr = s.Status.LoadBalancer.Ingress[0].Hostname
			}
			Expect(addr).Should(Not(BeEmpty()))

			By("connecting Hazelcast client")
			config := hzClient.Config{}
			config.Cluster.Network.SetAddresses(fmt.Sprintf("%s:5701", addr))
			config.Cluster.Unisocket = unisocket
			config.Cluster.Discovery.UsePublicIP = true
			client, err := hzClient.StartNewClientWithConfig(ctx, config)
			Expect(err).ToNot(HaveOccurred())

			By("using Hazelcast client")
			m, err := client.GetMap(ctx, "map")
			Expect(err).ToNot(HaveOccurred())
			for i := 0; i < 100; i++ {
				_, err = m.Put(ctx, strconv.Itoa(i), strconv.Itoa(i))
				Expect(err).ToNot(HaveOccurred())
			}
			err = client.Shutdown(ctx)
			Expect(err).ToNot(HaveOccurred())
		}

		assertExternalAddressesNotEmpty := func() {
			By("status external addresses should not be empty")
			Eventually(func() string {
				cr := hazelcastcomv1alpha1.Hazelcast{}
				err := k8sClient.Get(context.Background(), lookupKey, &cr)
				Expect(err).ToNot(HaveOccurred())
				return cr.Status.ExternalAddresses
			}, timeout, interval).Should(Not(BeEmpty()))
		}

		It("should create Hazelcast cluster and allow connecting with Hazelcast unisocket client", Label("slow"), func() {
			assertUseHazelcastUnisocket := func() {
				assertUseHazelcast(true)
			}

			hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzNamespace, ee)
			create(hazelcast)
			assertUseHazelcastUnisocket()
			assertExternalAddressesNotEmpty()
		})

		It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", Label("slow"), func() {
			assertUseHazelcastSmart := func() {
				assertUseHazelcast(false)
			}

			hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzNamespace, ee)
			create(hazelcast)
			assertUseHazelcastSmart()
			assertExternalAddressesNotEmpty()
		})

		It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", Label("slow"), func() {
			assertUseHazelcastSmart := func() {
				assertUseHazelcast(false)
			}
			hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzNamespace, ee)
			create(hazelcast)
			assertUseHazelcastSmart()
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", Label("fast"), func() {
			hazelcast := hazelcastconfig.ClusterName(hzNamespace, ee)
			create(hazelcast)

			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		})
	})

	Context("Hazelcast member status", func() {

		It("should update HZ ready members status", Label("fast"), func() {
			h := hazelcastconfig.Default(hzNamespace, ee)
			create(h)

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

		It("should update HZ detailed member status", Label("fast"), func() {
			h := hazelcastconfig.Default(hzNamespace, ee)
			create(h)

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
			create(hazelcast)

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
			create(hazelcast)

			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			By("Check the HotBackup creation sequence")
			logs := test.GetPodLogs(context.Background(), types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.PodLogOptions{
				Follow:    true,
				SinceTime: &v1.Time{Time: t},
			})
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
			create(hazelcast)

			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			By("Finding Backup sequence")
			logs := test.GetPodLogs(context.Background(), types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.PodLogOptions{
				Follow:    true,
				SinceTime: &v1.Time{Time: t},
			})
			defer logs.Close()
			scanner := bufio.NewScanner(logs)
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Starting new hot backup with sequence"))
			line := scanner.Text()
			Expect(logs.Close()).Should(Succeed())

			compRegEx := regexp.MustCompile(`Starting new hot backup with sequence (?P<seq>\d+)`)
			match := compRegEx.FindStringSubmatch(line)
			var seq string
			for i, name := range compRegEx.SubexpNames() {
				if name == "seq" && i > 0 && i <= len(match) {
					seq = match[i]
				}
			}
			if seq == "" {
				Fail("Backup sequence not found")
			}
			Expect(k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())

			assertDoesNotExist(types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.Pod{})

			By("Waiting for Hazelcast CR to be removed", func() {
				Eventually(func() error {
					h := &hazelcastcomv1alpha1.Hazelcast{}
					return k8sClient.Get(context.Background(), types.NamespacedName{
						Name:      hzName,
						Namespace: hzNamespace,
					}, h)
				}, timeout, interval).ShouldNot(Succeed())
			})

			By("Creating new Hazelcast cluster from existing backup with 2 members")
			baseDir := "/data/hot-restart/hot-backup/backup-" + seq
			hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, false)
			hazelcast.Spec.ClusterSize = &[]int32{2}[0]
			hazelcast.Spec.Persistence.DataRecoveryTimeout = 60
			hazelcast.Spec.Persistence.AutoForceStart = true
			create(hazelcast)

			evaluateReadyMembers(lookupKey, 2)
		})

		DescribeTable("should successfully restart from HotBackup data", func(params ...interface{}) {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			baseDir := "/data/hot-restart"
			hazelcast := addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, params...), getFirstWorkerNodeName())
			create(hazelcast)

			evaluateReadyMembers(lookupKey, 3)

			By("Creating HotBackup CR")
			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			By("Finding Backup sequence")
			logs := test.GetPodLogs(context.Background(), types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.PodLogOptions{
				Follow:    true,
				SinceTime: &v1.Time{Time: t},
			})
			defer logs.Close()
			scanner := bufio.NewScanner(logs)
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Starting new hot backup with sequence"))
			line := scanner.Text()
			Expect(logs.Close()).Should(Succeed())

			compRegEx := regexp.MustCompile(`Starting new hot backup with sequence (?P<seq>\d+)`)
			match := compRegEx.FindStringSubmatch(line)
			var seq string
			for i, name := range compRegEx.SubexpNames() {
				if name == "seq" && i > 0 && i <= len(match) {
					seq = match[i]
				}
			}
			if seq == "" {
				Fail("Backup sequence not found")
			}
			Expect(k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())

			assertDoesNotExist(types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.Pod{})

			By("Waiting for Hazelcast CR to be removed", func() {
				Eventually(func() error {
					h := &hazelcastcomv1alpha1.Hazelcast{}
					return k8sClient.Get(context.Background(), types.NamespacedName{
						Name:      hzName,
						Namespace: hzNamespace,
					}, h)
				}, timeout, interval).ShouldNot(Succeed())
			})

			By("Creating new Hazelcast cluster from existing backup")
			baseDir += "/hot-backup/backup-" + seq
			hazelcast = addNodeSelectorForName(hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir, params...), getFirstWorkerNodeName())

			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
			evaluateReadyMembers(lookupKey, 3)

			logs = test.GetPodLogs(context.Background(), types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.PodLogOptions{Follow: true})
			defer logs.Close()

			scanner = bufio.NewScanner(logs)
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
				h := &hazelcastcomv1alpha1.Hazelcast{}
				_ = k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      hzName,
					Namespace: hzNamespace,
				}, h)
				return h.Status.Restore
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
		It("should create Map Config", func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-1", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())

			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		})
		It("should create Map Config with correct default values", func() {
			localPort := "8000"
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)

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
			Expect(mapConfig.InMemoryFormat).Should(Equal(hazelcastcomv1alpha1.EncodeInMemoryFormat[codecTypes.InMemoryFormatBinary]))
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
		It("should create Map Config with Indexes", func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-2", hzNamespace)
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
					Attributes: []string{"attribute1", "attribute2"},
					BitmapIndexOptions: &hazelcastcomv1alpha1.BitmapIndexOptionsConfig{
						UniqueKey:           "key",
						UniqueKeyTransition: hazelcastcomv1alpha1.UniqueKeyTransitionRAW,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			// TODO: When Indexes can be decoded in the getMapConfig method, we can check if indexes are created correctly.
		})
		It("should fail when persistence of Map CR and Hazelcast CR do not match", func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)

			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-3", hzNamespace)
			m.Spec.PersistenceEnabled = true

			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
			Expect(m.Status.Message).To(Equal(fmt.Sprintf("Persistence is not enabled for the Hazelcast resource %s.", hazelcast.Name)))
		})
		It("should update the map correctly", func() {
			localPort := "8000"
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)

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
				EvictionPolicy: codecTypes.EvictionPolicyLFU,
				MaxSize:        pointer.Int32Ptr(500),
				MaxSizePolicy:  codecTypes.MaxSizePolicyFreeHeapSize,
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
		It("should fail to update", func() {
			hazelcast := hazelcastconfig.Default(hzNamespace, ee)
			create(hazelcast)

			By("creating the map config successfully")
			m := hazelcastconfig.DefaultMap(hazelcast.Name, "map-123", hzNamespace)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("failing to update map config")
			m.Spec.BackupCount = pointer.Int32Ptr(3)
			Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
		})

		It("should keep the entries after a Hot Backup", func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			localPort := "8000"
			mapName := "map123"
			baseDir := "/data/hot-restart"

			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir)
			create(hazelcast)
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

			By("Finding Backup sequence")
			logs := test.GetPodLogs(context.Background(), types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.PodLogOptions{
				Follow:    true,
				SinceTime: &v1.Time{Time: t},
			})
			defer logs.Close()
			scanner := bufio.NewScanner(logs)
			test.EventuallyInLogs(scanner, timeout, logInterval).
				Should(ContainSubstring("Starting new hot backup with sequence"))
			line := scanner.Text()
			Expect(logs.Close()).Should(Succeed())

			compRegEx := regexp.MustCompile(`Starting new hot backup with sequence (?P<seq>\d+)`)
			match := compRegEx.FindStringSubmatch(line)
			var seq string
			for i, name := range compRegEx.SubexpNames() {
				if name == "seq" && i > 0 && i <= len(match) {
					seq = match[i]
				}
			}
			if seq == "" {
				Fail("Backup sequence not found")
			}
			Expect(k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())

			assertDoesNotExist(types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.Pod{})

			By("Waiting for Hazelcast CR to be removed", func() {
				Eventually(func() error {
					h := &hazelcastcomv1alpha1.Hazelcast{}
					return k8sClient.Get(context.Background(), types.NamespacedName{
						Name:      hzName,
						Namespace: hzNamespace,
					}, h)
				}, timeout, interval).ShouldNot(Succeed())
			})

			By("Creating new Hazelcast cluster from existing backup")
			baseDir += "/hot-backup/backup-" + seq
			hazelcast = hazelcastconfig.PersistenceEnabled(hzNamespace, baseDir)

			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
			evaluateReadyMembers(lookupKey, 3)

			logs = test.GetPodLogs(context.Background(), types.NamespacedName{
				Name:      hzName + "-0",
				Namespace: hzNamespace,
			}, &corev1.PodLogOptions{Follow: true})
			defer logs.Close()

			scanner = bufio.NewScanner(logs)
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
			Expect(err).To((BeNil()))
			Expect(mp.Size(context.Background())).Should(Equal(entryCount))
		})
	})
})

func emptyHazelcast() *hazelcastcomv1alpha1.Hazelcast {
	return &hazelcastcomv1alpha1.Hazelcast{
		ObjectMeta: v1.ObjectMeta{
			Name:      hzName,
			Namespace: hzNamespace,
		},
	}
}

func isHazelcastRunning(hz *hazelcastcomv1alpha1.Hazelcast) bool {
	return hz.Status.Phase == "Running"
}

// assertMemberLogs check that the given expected string can be found in the logs.
// expected can be a regexp pattern.
func assertMemberLogs(h *hazelcastcomv1alpha1.Hazelcast, expected string) {
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      h.Name + "-0",
		Namespace: h.Namespace,
	}, &corev1.PodLogOptions{})
	defer logs.Close()
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		if match, _ := regexp.MatchString(expected, line); match {
			return
		}
	}
	Fail(fmt.Sprintf("Failed to find \"%s\" in member logs", expected))
}

func evaluateReadyMembers(lookupKey types.NamespacedName, membersCount int) {
	hz := &hazelcastcomv1alpha1.Hazelcast{}
	Eventually(func() string {
		err := k8sClient.Get(context.Background(), lookupKey, hz)
		Expect(err).ToNot(HaveOccurred())
		return hz.Status.Cluster.ReadyMembers
	}, timeout, interval).Should(Equal(fmt.Sprintf("%d/%d", membersCount, membersCount)))
}

func getFirstWorkerNodeName() string {
	labelMatcher := client.MatchingLabels{}
	if platform.GetPlatform().Type == platform.OpenShift {
		labelMatcher = client.MatchingLabels{
			"node-role.kubernetes.io/worker": "",
		}
	}
	nodes := &corev1.NodeList{}
	Expect(k8sClient.List(context.Background(), nodes, labelMatcher)).Should(Succeed())
loop1:
	for _, node := range nodes.Items {
		for _, taint := range node.Spec.Taints {
			if taint.Key == "node.kubernetes.io/unreachable" {
				continue loop1
			}
		}
		return node.ObjectMeta.Name
	}
	Fail("Could not find a reachable working node.")
	return ""
}

func addNodeSelectorForName(hz *hazelcastcomv1alpha1.Hazelcast, n string) *hazelcastcomv1alpha1.Hazelcast {
	// If hostPath is not enabled, do nothing
	if hz.Spec.Scheduling == nil {
		return hz
	}
	// If NodeSelector is set with dummy name, put the real node name
	if hz.Spec.Scheduling.NodeSelector != nil {
		hz.Spec.Scheduling.NodeSelector = map[string]string{"kubernetes.io/hostname": n}
	}
	return hz
}

func createHazelcastClient(ctx context.Context, h *hazelcastcomv1alpha1.Hazelcast, localPort string) *hzClient.Client {
	config := hzClient.Config{}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses("localhost:" + localPort)
	client, err := hzClient.StartNewClientWithConfig(ctx, config)
	Expect(err).To(BeNil())
	return client

}

func getMapConfig(ctx context.Context, client *hzClient.Client, mapName string) codecTypes.MapConfig {
	ci := hzClient.NewClientInternal(client)
	req := codec.EncodeMCGetMapConfigRequest(mapName)
	resp, err := ci.InvokeOnRandomTarget(ctx, req, nil)
	Expect(err).To(BeNil())
	return codec.DecodeMCGetMapConfigResponse(resp)
}

func portForwardPod(sName, sNamespace, port string) (chan struct{}, chan struct{}) {
	defer GinkgoRecover()
	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	Expect(err).To(BeNil())

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	Expect(err).To(BeNil())

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", sNamespace, sName)
	hostIP := strings.TrimPrefix(config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{port}, stopChan, readyChan, out, errOut)
	Expect(err).To(BeNil())

	go func() {
		if err := forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
			GinkgoWriter.Println(err.Error())
			Expect(err).To(BeNil())
		}
	}()

	return stopChan, readyChan

}

func waitForReadyChannel(readyChan chan struct{}, dur time.Duration) error {
	timer := time.NewTimer(dur)
	for {
		select {
		case <-readyChan:
			return nil
		case <-timer.C:
			return fmt.Errorf("Timeout waiting for readyChannel")
		}
	}
}
func closeChannel(closeChan chan struct{}) {
	closeChan <- struct{}{}
}

func assertMapStatus(m *hazelcastcomv1alpha1.Map, st hazelcastcomv1alpha1.MapConfigState) *hazelcastcomv1alpha1.Map {
	checkMap := &hazelcastcomv1alpha1.Map{}
	By("Waiting for Map CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.MapConfigState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      m.Name,
				Namespace: m.Namespace,
			}, checkMap)
			if err != nil {
				return ""
			}
			return checkMap.Status.State
		}, timeout, interval).Should(Equal(st))
	})
	return checkMap
}
