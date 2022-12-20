package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	hz "github.com/hazelcast/hazelcast-go-client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Cache Config with Persistence", Label("cache_persistence"), func() {
	localPort := strconv.Itoa(8000 + GinkgoParallelProcess())
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
				GinkgoWriter.Printf("%+v\n", controllerManagerName)
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1alpha1.HotBackup{}, &hazelcastv1alpha1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Cache{}, &hazelcastv1alpha1.CacheList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should fail when persistence of Cache CR and Hazelcast CR do not match", Label("fast"), func() {
		setLabelAndCRName("hchp-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true

		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertDataStructureStatus(chLookupKey, hazelcastv1alpha1.DataStructureFailed, m)
		Expect(m.Status.Message).To(Equal(fmt.Sprintf("persistence is not enabled for the Hazelcast resource %s", hazelcast.Name)))
	})

	It("should keep the entries after a Hot Backup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hchp-2")
		clusterSize := int32(3)

		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the cache config")
		cache := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
		cache.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), cache)).Should(Succeed())
		assertDataStructureStatus(chLookupKey, hazelcastv1alpha1.DataStructureSuccess, cache)

		By("filling the cache with entries")
		entryCount := 10
		fillCachePortForward(hazelcast, cache.GetDSName(), localPort, entryCount)
		validateCacheEntriesPortForward(hazelcast, localPort, cache.GetDSName(), entryCount)

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 1*Minute)

		By("filling the cache with entries after backup")
		fillCachePortForward(hazelcast, cache.GetDSName(), localPort, entryCount)

		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = &hazelcastv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}

		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey)
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1alpha1.RestoreSucceeded)

		By("checking the cache entries")
		validateCacheEntriesPortForward(hazelcast, localPort, cache.GetDSName(), entryCount)
	})

	It("should persist the cache successfully created configs into the configmap", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hchp-3")
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
				assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastv1alpha1.DataStructureFailed, c)
				continue
			}
			assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastv1alpha1.DataStructureSuccess, c)
		}

		By("checking if the caches are in the ConfigMap", func() {
			assertCacheConfigsPersisted(hazelcast, "cache1", "cache2", "cache3")
		})

		By("deleting cache2")
		Expect(k8sClient.Delete(context.Background(),
			&hazelcastv1alpha1.Cache{ObjectMeta: v1.ObjectMeta{Name: "cache2", Namespace: hazelcast.Namespace}})).Should(Succeed())

		By("checking if cache2 is not persisted in the configmap", func() {
			assertCacheConfigsPersisted(hazelcast, "cache1", "cache3")
		})
	})

	It("should continue persisting last applied Cache Config in case of failure", Label("fast"), func() {
		setLabelAndCRName("hchp-4")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		c := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
		assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastv1alpha1.DataStructureSuccess, c)

		By("checking if the cache config is persisted")
		hzConfig := assertCacheConfigsPersisted(hazelcast, c.Name)
		ccfg := hzConfig.Hazelcast.Cache[c.Name]

		By("failing to update the cache config")
		c.Spec.BackupCount = pointer.Int32(4)
		Expect(k8sClient.Update(context.Background(), c)).Should(Succeed())
		assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastv1alpha1.DataStructureFailed, c)

		By("checking if the same cache config is still there")
		// Should wait for Hazelcast reconciler to get triggered, we do not have a waiting mechanism for that.
		Sleep(5 * Second)
		hzConfig = assertCacheConfigsPersisted(hazelcast, c.Name)
		newCcfg := hzConfig.Hazelcast.Cache[c.Name]
		Expect(newCcfg).To(Equal(ccfg))
	})
})

func validateCacheEntriesPortForward(h *hazelcastv1alpha1.Hazelcast, localPort, cacheName string, entryCount int) {
	stopChan := portForwardPod(h.Name+"-0", h.Namespace, localPort+":5701")
	defer closeChannel(stopChan)
	cl := newHazelcastClientPortForward(context.Background(), h, localPort)
	cli := hz.NewClientInternal(cl)
	for i := 0; i < entryCount; i++ {
		key, err := cli.EncodeData(fmt.Sprintf("mykey%d", i))
		Expect(err).To(BeNil())
		value := fmt.Sprintf("myvalue%d", i)
		getRequest := codec.EncodeCacheGetRequest("/hz/"+cacheName, key, nil)
		resp, err := cli.InvokeOnKey(context.Background(), getRequest, key, nil)
		pairs := codec.DecodeCacheGetResponse(resp)
		Expect(err).To(BeNil())
		data, err := cli.DecodeData(pairs)
		Expect(err).To(BeNil())
		Expect(fmt.Sprintf("%v", data)).Should(Equal(value))
	}
}

func fillCachePortForward(h *hazelcastv1alpha1.Hazelcast, cacheName, localPort string, entryCount int) {
	stopChan := portForwardPod(h.Name+"-0", h.Namespace, localPort+":5701")
	defer closeChannel(stopChan)
	cl := newHazelcastClientPortForward(context.Background(), h, localPort)
	cli := hz.NewClientInternal(cl)

	for _, mi := range cli.OrderedMembers() {
		configRequest := codec.EncodeCacheGetConfigRequest("/hz/"+cacheName, cacheName)
		_, _ = cli.InvokeOnMember(context.Background(), configRequest, mi.UUID, nil)
	}

	for i := 0; i < entryCount; i++ {
		key, err := cli.EncodeData(fmt.Sprintf("mykey%d", i))
		Expect(err).To(BeNil())
		value, err := cli.EncodeData(fmt.Sprintf("myvalue%d", i))
		Expect(err).To(BeNil())
		cpr := codec.EncodeCachePutRequest("/hz/"+cacheName, key, value, nil, false, 0)
		_, err = cli.InvokeOnKey(context.Background(), cpr, key, nil)
		Expect(err).To(BeNil())
	}
}
