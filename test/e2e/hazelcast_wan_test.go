package e2e

import (
	"context"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast WAN", Label("hz_wan"), func() {
	waitForLBAddress := func(name types.NamespacedName) string {
		By("waiting for load balancer address")
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() string {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(context.Background(), name, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.ExternalAddresses
		}, 3*Minute, interval).Should(Not(BeEmpty()))
		Expect(k8sClient.Get(context.Background(), name, hz)).To(Succeed())
		return hz.Status.ExternalAddresses
	}

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, hzNamespace, labels)
	})

	It("should send data to another cluster", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hw-1")
		// Create source and target Hazelcast clusters
		hazelcastSource := hazelcastconfig.ExposeExternallyUnisocket(hzSourceLookupKey, ee, labels)
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)
		hazelcastTarget := hazelcastconfig.ExposeExternallyUnisocket(hzTargetLookupKey, ee, labels)
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		evaluateReadyMembers(hzSourceLookupKey, 3)
		evaluateReadyMembers(hzTargetLookupKey, 3)

		_ = waitForLBAddress(hzSourceLookupKey)
		targetAddress := waitForLBAddress(hzTargetLookupKey)

		// Create map for source Hazelcast cluster
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcastSource.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		// Create WAN configuration for the map
		By("creating WAN configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			wanLookupKey,
			m.Name,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wan := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), wanLookupKey, wan)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wan.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		// Fill the map in source cluster
		mapSize := 1024
		By("filling the source")
		FillTheMapData(context.Background(), hzSourceLookupKey, true, m.Name, mapSize)

		// Wait for data to appear in target cluster
		By("checking the size of the map in the target cluster")
		waitForMapSize(context.Background(), hzTargetLookupKey, m.Name, mapSize)
	})
})
