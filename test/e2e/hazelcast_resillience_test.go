package e2e

import (
	"context"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Resilience", Label("slow"), func() {
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should be able to reconnect to Hazelcast cluster upon restart even when Hazelcast cluster is marked to be deleted", Label("slow"), func() {
		By("creating source Hazelcast cluster")
		hazelcastSource := hazelcastconfig.Default(hzSrcLookupKey, ee, labels)
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.Default(hzTrgLookupKey, ee, labels)
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)

		evaluateReadyMembers(hzSrcLookupKey)
		evaluateReadyMembers(hzTrgLookupKey)

		By("creating map for source Hazelcast cluster")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcastSource.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("creating wan replication configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			wanLookupKey,
			m.Name,
			hazelcastTarget.Spec.ClusterName,
			hzclient.HazelcastUrl(hazelcastTarget),
			labels,
		)
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())

		By("deleting operator")
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(context.Background(), controllerManagerName, dep)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), dep, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
		assertDoesNotExist(controllerManagerName, &appsv1.Deployment{})

		By("deleting Hazelcast clusters")
		Expect(k8sClient.Delete(context.Background(), hazelcastSource)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), hazelcastTarget)).Should(Succeed())

		By("deleting wan replication")
		Expect(k8sClient.Delete(context.Background(), wan)).Should(Succeed())

		By("creating operator again")
		newDep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dep.Name,
				Namespace: dep.Namespace,
				Labels:    dep.Labels,
			},
			Spec: dep.Spec,
		}
		Expect(k8sClient.Create(context.Background(), newDep)).Should(Succeed())
		Eventually(func() (int32, error) {
			return getDeploymentReadyReplicas(context.Background(), controllerManagerName, newDep)
		}, 90*Second, interval).Should(Equal(int32(1)))

		assertDoesNotExist(mapLookupKey, m)
		assertDoesNotExist(wanLookupKey, wan)
		assertDoesNotExist(hzSrcLookupKey, hazelcastSource)
		assertDoesNotExist(hzTrgLookupKey, hazelcastTarget)
	})
})
