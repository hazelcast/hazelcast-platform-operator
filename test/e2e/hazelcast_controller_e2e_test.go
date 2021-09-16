package e2e

import (
	"context"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
		waitDeletionOf(lookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	Describe("Creating CR", func() {

		It("Should create Hazelcast CR with default values", func() {

			By("Checking hazelcast-enterprise-controller-manager running", func() {
				controllerDep := &appsv1.Deployment{}
				Eventually(func() (int32, error) {
					return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
				}, timeout, interval).Should(Equal(int32(1)))
			})

			hazelcast := emptyHazelcast()
			err := loadFromFile(hazelcast, "_v1alpha1_hazelcast.yaml")
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
