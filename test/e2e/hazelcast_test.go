package e2e

import (
	"context"
	"fmt"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

const (
	logInterval = 10 * Millisecond
)

var _ = Describe("Hazelcast", Label("hz"), func() {
	hzName := fmt.Sprintf("hz-%d", GinkgoParallelProcess())
	var hzLookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}
	labels := map[string]string{
		"test_suite": fmt.Sprintf("hz-%d", GinkgoParallelProcess()),
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

	Describe("Default Hazelcast CR", func() {
		It("should create Hazelcast cluster", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", Label("fast"), func() {
			hazelcast := hazelcastconfig.ClusterName(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		})
	})

	Context("Hazelcast member status", func() {
		It("should update HZ ready members status", Label("slow"), func() {
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey, 3)
			assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				deletePods(hzLookupKey)
				evaluateReadyMembers(hzLookupKey, 3)
			})
		})

		It("should update HZ detailed member status", Label("fast"), func() {
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey, 3)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			memberStateT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) string {
				return status.State
			}
			masterT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) bool {
				return status.Master
			}
			Eventually(func() []hazelcastcomv1alpha1.HazelcastMemberStatus {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Members
			}, 30*Second, interval).Should(And(HaveLen(3),
				ContainElement(WithTransform(memberStateT, Equal("ACTIVE"))),
				ContainElement(WithTransform(masterT, Equal(true))),
			))
		})
	})

	Describe("External API errors", func() {
		assertStatusAndMessageEventually := func(phase hazelcastcomv1alpha1.Phase) {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Phase
			}, 30*Second, interval).Should(Equal(phase))
			Expect(hz.Status.Message).Should(Not(BeEmpty()))
		}

		It("should be reflected to Hazelcast CR status", Label("fast"), func() {
			CreateHazelcastCRWithoutCheck(hazelcastconfig.Faulty(hzLookupKey, ee, labels))
			assertStatusAndMessageEventually(hazelcastcomv1alpha1.Failed)
		})
	})

})
