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

var _ = Describe("Hazelcast MultiMap Config", Label("multimap"), func() {
	localPort := strconv.Itoa(8300 + GinkgoParallelProcess())

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
		DeleteAllOf(&hazelcastcomv1alpha1.MultiMap{}, &hazelcastcomv1alpha1.MultiMapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create MultiMap Config", Label("fast"), func() {
		setLabelAndCRName("hmm-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		mm := hazelcastconfig.DefaultMultiMap(mmLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
		assertDataStructureStatus(mmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.MultiMap{})
	})

	It("should create MultiMap Config with correct default values", Label("fast"), func() {
		setLabelAndCRName("hmm-2")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the default multiMap config")
		mm := hazelcastconfig.DefaultMultiMap(mmLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
		mm = assertDataStructureStatus(mmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.MultiMap{}).(*hazelcastcomv1alpha1.MultiMap)

		memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
		multiMapConfig := getMultiMapConfigFromMemberConfig(memberConfigXML, mm.GetDSName())
		Expect(multiMapConfig).NotTo(BeNil())

		Expect(multiMapConfig.BackupCount).Should(Equal(n.DefaultMultiMapBackupCount))
		Expect(multiMapConfig.Binary).Should(Equal(n.DefaultMultiMapBinary))
		Expect(multiMapConfig.CollectionType).Should(Equal(n.DefaultMultiMapCollectionType))
	})

	It("should fail to update MultiMap Config", Label("fast"), func() {
		setLabelAndCRName("hmm-3")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the multiMap config")
		mms := hazelcastcomv1alpha1.MultiMapSpec{
			DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
				HazelcastResourceName: hzLookupKey.Name,
				BackupCount:           pointer.Int32(3),
			},
			Binary:         true,
			CollectionType: hazelcastcomv1alpha1.CollectionTypeList,
		}
		mm := hazelcastconfig.MultiMap(mms, mmLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
		mm = assertDataStructureStatus(mmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.MultiMap{}).(*hazelcastcomv1alpha1.MultiMap)

		By("failing to update multiMap config")
		mm.Spec.BackupCount = pointer.Int32(5)
		mm.Spec.Binary = false
		Expect(k8sClient.Update(context.Background(), mm)).Should(Succeed())
		assertDataStructureStatus(mmLookupKey, hazelcastcomv1alpha1.DataStructureFailed, &hazelcastcomv1alpha1.MultiMap{})
	})
})
