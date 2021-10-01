package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	hzClient "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		Name:      "hazelcast-enterprise-controller-manager",
		Namespace: hzNamespace,
	}

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("Checking hazelcast-enterprise-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, timeout, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
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

	Describe("Default Hazelcast CR", func() {
		It("should create Hazelcast cluster", func() {
			hazelcast := loadHazelcast("default.yaml")
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
			ip := s.Status.LoadBalancer.Ingress[0].IP
			Expect(ip).Should(Not(Equal("")))

			By("connecting Hazelcast client")
			config := hzClient.Config{}
			config.Cluster.Network.SetAddresses(fmt.Sprintf("%s:5701", ip))
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

			hazelcast := loadHazelcast("expose_externally_unisocket.yaml")
			create(hazelcast)
			assertUseHazelcastUnisocket()
		})

		It("should create Hazelcast cluster exposed with NodePort services and allow connecting with Hazelcast smart client", func() {
			assertUseHazelcastSmart := func() {
				assertUseHazelcast(false)
			}

			hazelcast := loadHazelcast("expose_externally_smart_nodeport.yaml")
			create(hazelcast)
			assertUseHazelcastSmart()
		})

		It("should create Hazelcast cluster exposed with LoadBalancer services and allow connecting with Hazelcast smart client", func() {
			assertUseHazelcastSmart := func() {
				assertUseHazelcast(false)
			}
			hazelcast := loadHazelcast("expose_externally_smart_loadbalancer.yaml")
			create(hazelcast)
			assertUseHazelcastSmart()
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", func() {
			hazelcast := loadHazelcast("cluster_name.yaml")
			create(hazelcast)
			logs := getPodLogs(context.Background(), types.NamespacedName{
				Name:      hazelcast.Name + "-0",
				Namespace: hazelcast.Namespace,
			})
			defer logs.Close()

			scanner := bufio.NewScanner(logs)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "Cluster name: "+hazelcast.Spec.ClusterName) {
					return
				}
			}
			Fail("Cluster name " + hazelcast.Spec.ClusterName + " not found in the logs")
		})
	})
})

func loadHazelcast(fileName string) *hazelcastcomv1alpha1.Hazelcast {
	h := emptyHazelcast()

	f, err := os.Open(fmt.Sprintf("config/hazelcast/%s", fileName))
	Expect(err).ToNot(HaveOccurred())
	defer f.Close()

	decoder := yaml.NewYAMLToJSONDecoder(f)
	err = decoder.Decode(h)
	Expect(err).ToNot(HaveOccurred())

	return h
}

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

func getPodLogs(ctx context.Context, pod types.NamespacedName) io.ReadCloser {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	p, err := clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
	if err != nil {
		panic(err)
	}
	if p.Status.Phase != corev1.PodFailed && p.Status.Phase != corev1.PodRunning {
		panic("Unable to get pod logs for the pod in Phase " + p.Status.Phase)
	}
	podLogOptions := corev1.PodLogOptions{}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOptions)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		panic(err)
	}
	return podLogs
}
