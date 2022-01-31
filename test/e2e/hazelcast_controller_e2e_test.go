package e2e

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/test"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/ginkgo"
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
	hzName = "hazelcast"
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
		It("should create Hazelcast cluster", func() {
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

		It("should create Hazelcast cluster and allow connecting with Hazelcast unisocket client", func() {
			assertUseHazelcastUnisocket := func() {
				assertUseHazelcast(true)
			}

			hazelcast := hazelcastconfig.ExposeExternallyUnisocket(hzNamespace, ee)
			create(hazelcast)
			assertUseHazelcastUnisocket()
		})

		It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", func() {
			assertUseHazelcastSmart := func() {
				assertUseHazelcast(false)
			}

			hazelcast := hazelcastconfig.ExposeExternallySmartNodePort(hzNamespace, ee)
			create(hazelcast)
			assertUseHazelcastSmart()
		})

		It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", func() {
			assertUseHazelcastSmart := func() {
				assertUseHazelcast(false)
			}
			hazelcast := hazelcastconfig.ExposeExternallySmartLoadBalancer(hzNamespace, ee)
			create(hazelcast)
			assertUseHazelcastSmart()
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", func() {
			hazelcast := hazelcastconfig.ClusterName(hzNamespace, ee)
			create(hazelcast)

			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		})
	})

	Context("Hazelcast member status", func() {

		It("should update HZ ready members status", func() {
			h := hazelcastconfig.Default(hzNamespace, ee)
			create(h)

			evaluateReadyMembers(lookupKey)

			assertMemberLogs(h, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				err := k8sClient.DeleteAllOf(context.Background(), &corev1.Pod{}, client.InNamespace(lookupKey.Namespace), client.MatchingLabels{
					n.ApplicationNameLabel:         n.Hazelcast,
					n.ApplicationInstanceNameLabel: h.Name,
					n.ApplicationManagedByLabel:    n.OperatorName,
				})
				Expect(err).ToNot(HaveOccurred())
				evaluateReadyMembers(lookupKey)
			})
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
		if !ee {
			Skip("This tests block will only run in EE configuration")
		}
		It("should enable persistence for members successfully", func() {
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace)
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
					Name: n.PersistencePvcName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: n.PersistencePvcName + "-" + pod.Name,
						},
					},
				}))
			}
		})

		It("should successfully trigger HotBackup", func() {
			hazelcast := hazelcastconfig.PersistenceEnabled(hzNamespace)
			create(hazelcast)

			evaluateReadyMembers(lookupKey)

			t := time.Now()
			hotBackup := hazelcastconfig.HotBackup(hazelcast.Name, hzNamespace)
			By("Creating HotBackup CR")
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
			test.EventuallyInLogs(scanner, timeout, interval).
				Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
			test.EventuallyInLogs(scanner, timeout, interval).
				Should(ContainSubstring("Starting new hot backup with sequence"))
			test.EventuallyInLogs(scanner, timeout, interval).
				Should(MatchRegexp("Backup of hot restart store \\S+ finished"))
			test.EventuallyInLogs(scanner, timeout, interval).
				Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"))
			Expect(logs.Close()).Should(Succeed())
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
		println(line)
		if match, _ := regexp.MatchString(expected, line); match {
			return
		}
	}
	Fail(fmt.Sprintf("Failed to find \"%s\" in member logs", expected))
}

func evaluateReadyMembers(lookupKey types.NamespacedName) {
	hz := &hazelcastcomv1alpha1.Hazelcast{}
	Eventually(func() string {
		err := k8sClient.Get(context.Background(), lookupKey, hz)
		Expect(err).ToNot(HaveOccurred())
		return hz.Status.Cluster.ReadyMembers
	}, timeout, interval).Should(Equal("3/3"))
}
