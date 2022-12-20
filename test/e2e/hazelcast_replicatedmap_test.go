package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast ReplicatedMap Config", Label("replicatedmap"), func() {
	localPort := strconv.Itoa(8600 + GinkgoParallelProcess())

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
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.ReplicatedMap{}, &hazelcastcomv1alpha1.ReplicatedMapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create ReplicatedMap Config", Label("fast"), func() {
		setLabelAndCRName("hrm-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		rm := hazelcastconfig.DefaultReplicatedMap(rmLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
		assertDataStructureStatus(rmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.ReplicatedMap{})
	})

	It("should create ReplicatedMap Config with correct default values", Label("fast"), func() {
		setLabelAndCRName("hrm-2")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the default ReplicatedMap config")
		rm := hazelcastconfig.DefaultReplicatedMap(rmLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
		rm = assertDataStructureStatus(rmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.ReplicatedMap{}).(*hazelcastcomv1alpha1.ReplicatedMap)

		memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
		replicatedMapConfig := getReplicatedMapConfigFromMemberConfig(memberConfigXML, rm.GetDSName())
		Expect(replicatedMapConfig).NotTo(BeNil())

		Expect(replicatedMapConfig.InMemoryFormat).Should(Equal(n.DefaultReplicatedMapInMemoryFormat))
		Expect(replicatedMapConfig.AsyncFillup).Should(Equal(n.DefaultReplicatedMapAsyncFillup))
	})

	It("should fail to update ReplicatedMap Config", Label("fast"), func() {
		setLabelAndCRName("hrm-3")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the ReplicatedMap config")
		rms := hazelcastcomv1alpha1.ReplicatedMapSpec{
			HazelcastResourceName: hzLookupKey.Name,
			InMemoryFormat:        hazelcastcomv1alpha1.InMemoryFormatBinary,
			AsyncFillup:           pointer.Bool(false),
		}
		rm := hazelcastconfig.ReplicatedMap(rms, rmLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
		rm = assertDataStructureStatus(rmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.ReplicatedMap{}).(*hazelcastcomv1alpha1.ReplicatedMap)

		By("failing to update ReplicatedMap config")
		rm.Spec.InMemoryFormat = hazelcastcomv1alpha1.InMemoryFormatObject
		rm.Spec.AsyncFillup = pointer.Bool(true)
		Expect(k8sClient.Update(context.Background(), rm)).Should(Succeed())
		assertDataStructureStatus(rmLookupKey, hazelcastcomv1alpha1.DataStructureFailed, &hazelcastcomv1alpha1.ReplicatedMap{})
	})
})
