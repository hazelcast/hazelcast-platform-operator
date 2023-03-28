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
		DeleteAllOf(&hazelcastcomv1alpha1.Cache{}, &hazelcastcomv1alpha1.CacheList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create Cache Config", Label("fast"), func() {
		setLabelAndCRName("hch-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		c := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
		assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Cache{})
	})

	It("should create Cache Config with correct default values", Label("fast"), func() {
		setLabelAndCRName("hch-2")
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

})
