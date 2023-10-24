package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Cache Config", Label("cache"), func() {
	localPort := strconv.Itoa(8000 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Cache{}, &hazelcastcomv1alpha1.CacheList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create Cache Config with correct default values", Label("fast"), func() {
		setLabelAndCRName("hch-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the default cache config")
		c := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
		c = assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Cache{}).(*hazelcastcomv1alpha1.Cache)

		By("checking if the cache config is created correctly")
		memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
		cacheConfig := getCacheConfigFromMemberConfig(memberConfigXML, c.GetDSName())
		Expect(cacheConfig).NotTo(BeNil())

		Expect(cacheConfig.BackupCount).Should(Equal(n.DefaultCacheBackupCount))
		Expect(cacheConfig.AsyncBackupCount).Should(Equal(n.DefaultCacheAsyncBackupCount))
		Expect(cacheConfig.StatisticsEnabled).Should(Equal(n.DefaultCacheStatisticsEnabled))
		Expect(string(cacheConfig.InMemoryFormat)).Should(Equal(string(c.Spec.InMemoryFormat)))
	})

	When("Native Memory is not enabled for Hazelcast CR", func() {
		It("should fail with InMemoryFormat value is set to NativeMemory", Label("fast"), func() {
			setLabelAndCRName("hch-2")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			By("creating the cache config with NativeMemory")
			c := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
			c.Spec.InMemoryFormat = hazelcastcomv1alpha1.InMemoryFormatNative

			Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
			c = assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureFailed, &hazelcastcomv1alpha1.Cache{}).(*hazelcastcomv1alpha1.Cache)

			Expect(c.Status.Message).To(ContainSubstring("Native Memory must be enabled at Hazelcast"))
		})
	})

})
