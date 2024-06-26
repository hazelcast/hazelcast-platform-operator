package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast MultiMap Config", Group("multimap"), func() {
	localPort := strconv.Itoa(8300 + GinkgoParallelProcess())

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

	Context("Creating MultiMap configurations", func() {
		It("creates MultiMap config with correct default values", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("mm-1")
			hazelcast := hazelcastconfig.Default(hzLookupKey, labels)
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
	})

	Context("Updating MultiMap configurations", func() {
		It("verifies that MultiMap Config updates are prohibited", Label("cluster_scope"), Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("mm-2")
			hazelcast := hazelcastconfig.Default(hzLookupKey, labels)
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
			Expect(k8sClient.Update(context.Background(), mm)).
				Should(MatchError(ContainSubstring("spec: Forbidden: cannot be updated")))
		})
	})
})
