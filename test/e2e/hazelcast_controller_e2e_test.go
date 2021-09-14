package e2e

import (
	"context"
	"fmt"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	hzClient "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
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

		By("Checking hazelcast-enterprise-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, timeout, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())

		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), lookupKey, &hazelcastcomv1alpha1.Hazelcast{})
			if err == nil {
				return false
			}
			return errors.IsNotFound(err)
		}, deleteTimeout, interval).Should(BeTrue())
	})

	Create := func(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
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

	Describe("default CR", func() {
		It("should create default Hazelcast CR", func() {
			hazelcast := defaultHazelcast()
			Create(hazelcast)
		})
	})

	Describe("expose externally feature", func() {
		AssertUseHazelcast := func(unisocket bool) {
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
			client, err := hzClient.StartNewClientWithConfig(ctx, config)
			Expect(err).ToNot(HaveOccurred())

			By("using Hazelcast client")
			m, err := client.GetMap(ctx, "map")
			Expect(err).ToNot(HaveOccurred())
			for i := 0; i < 100; i++ {
				_, err = m.Put(ctx, strconv.Itoa(i), strconv.Itoa(i))
				Expect(err).ToNot(HaveOccurred())
			}
			client.Shutdown(ctx)
		}

		Context("unisocket client", func() {
			AssertUseHazelcastUnisocket := func() {
				AssertUseHazelcast(true)
			}

			It("should use Hazelcast cluster", func() {
				hazelcast := defaultHazelcast()
				hazelcast.Spec.ExposeExternally = hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				}
				Create(hazelcast)
				AssertUseHazelcastUnisocket()
			})
		})

		Context("smart client", func() {
			AssertUseHazelcastSmart := func() {
				AssertUseHazelcast(false)
			}

			Context("each member exposed via NodePort service", func() {
				It("should use Hazelcast cluster", func() {
					hazelcast := defaultHazelcast()
					hazelcast.Spec.ExposeExternally = hazelcastcomv1alpha1.ExposeExternallyConfiguration{
						Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
						DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
						MemberAccess:         hazelcastcomv1alpha1.MemberAccessNodePortExternalIP,
					}
					Create(hazelcast)
					AssertUseHazelcastSmart()
				})
			})

			Context("each member exposed via LoadBalancer service", func() {
				It("should use Hazelcast cluster", func() {
					hazelcast := defaultHazelcast()
					hazelcast.Spec.ExposeExternally = hazelcastcomv1alpha1.ExposeExternallyConfiguration{
						Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
						DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
						MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
					}
					Create(hazelcast)
					AssertUseHazelcastSmart()
				})
			})
		})
	})
})

func useExistingCluster() bool {
	return strings.ToLower(os.Getenv("USE_EXISTING_CLUSTER")) == "true"
}

func getDeploymentReadyReplicas(ctx context.Context, name types.NamespacedName, deploy *appsv1.Deployment) (int32, error) {
	err := k8sClient.Get(ctx, name, deploy)
	if err != nil {
		if errors.IsNotFound(err) {
			return 0, nil
		}
		return 0, err
	}

	return deploy.Status.ReadyReplicas, nil
}

func defaultHazelcast() *hazelcastcomv1alpha1.Hazelcast {
	h := emptyHazelcast()
	err := loadHazelcastFromFile(h, "_v1alpha1_hazelcast.yaml")
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

func loadHazelcastFromFile(hazelcast *hazelcastcomv1alpha1.Hazelcast, fileName string) error {
	f, err := os.Open(fmt.Sprintf("../../config/samples/%s", fileName))
	if err != nil {
		return err
	}
	defer f.Close()

	return decodeYAML(f, hazelcast)
}

func decodeYAML(r io.Reader, obj interface{}) error {
	decoder := yaml.NewYAMLToJSONDecoder(r)
	return decoder.Decode(obj)
}

func isHazelcastRunning(hz *hazelcastcomv1alpha1.Hazelcast) bool {
	if hz.Status.Phase == "Running" {
		return true
	} else {
		return false
	}
}
