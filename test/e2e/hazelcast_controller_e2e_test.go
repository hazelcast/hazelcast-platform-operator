package e2e

import (
	"context"
	"fmt"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	Describe("Creating CR", func() {

		It("Creating Hazelcast CR with default values", func() {

			By("Checking hazelcast-enterprise-controller-manager running", func() {
				controllerDep := &appsv1.Deployment{}
				Eventually(func() (int32, error) {
					return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
				}, timeout, interval).Should(Equal(int32(1)))
			})

			hazelcast := emptyHazelcast()
			err := loadHazelcastFromFile(hazelcast, "_v1alpha1_hazelcast.yaml")
			Expect(err).ToNot(HaveOccurred())

			By("Creating Hazelcast CR", func() {
				Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
			})

			By("Checking Hazelcast CR running", func() {
				hz := &hazelcastcomv1alpha1.Hazelcast{}
				Eventually(func() bool {
					k8sClient.Get(context.Background(), lookupKey, hz)
					return isHazelcastRunning(hz)
				}, timeout, interval).Should(BeTrue())
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
