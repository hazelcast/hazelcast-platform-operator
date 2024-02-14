package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Cache Config", Group("cache"), func() {
	localPort := strconv.Itoa(8000 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Cache{}, &hazelcastcomv1alpha1.CacheList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, &hazelcastcomv1alpha1.HotBackupList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Creating cache configurations", func() {
		It("should successfully create a cache config with correct default settings", Tag(Fast|Any), func() {
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

		It("should persist and remove cache config in/from Hazelcast config", Tag(Fast|EE|AnyCloud), func() {
			setLabelAndCRName("hch-2")
			caches := []string{"cache1", "cache2", "cache3", "cachefail"}
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)
			By("creating the cache configs")
			for _, cache := range caches {
				c := hazelcastconfig.DefaultCache(types.NamespacedName{Name: cache, Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
				c.Spec.HazelcastResourceName = hazelcast.Name
				if cache == "cachefail" {
					c.Spec.HazelcastResourceName = "failedHz"
				}
				Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
				if cache == "cachefail" {
					assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastcomv1alpha1.DataStructureFailed, c)
					continue
				}
				assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastcomv1alpha1.DataStructureSuccess, c)
			}
			By("checking if the caches are in the Config", func() {
				assertCacheConfigsPersisted(hazelcast, "cache1", "cache2", "cache3")
			})
			By("deleting cache2")
			Expect(k8sClient.Delete(context.Background(),
				&hazelcastcomv1alpha1.Cache{ObjectMeta: v1.ObjectMeta{Name: "cache2", Namespace: hazelcast.Namespace}})).Should(Succeed())
			By("checking if cache2 is not persisted in the Config", func() {
				assertCacheConfigsPersisted(hazelcast, "cache1", "cache3")
			})
		})
	})

	Context("Validating cache configurations", func() {
		When("Native Memory is not enabled for Hazelcast CR", func() {
			It("should fail to create a cache config with InMemoryFormatNative", Tag(Fast|Any), func() {
				setLabelAndCRName("hch-3")
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

		It("should fail due to mismatch in persistence settings between Cache CR and Hazelcast CR", Tag(Fast|Any), func() {
			setLabelAndCRName("hch-4")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			m := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
			m.Spec.PersistenceEnabled = true
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureFailed, m)
			Expect(m.Status.Message).To(ContainSubstring("Persistence must be enabled at Hazelcast"))
		})
	})

})
