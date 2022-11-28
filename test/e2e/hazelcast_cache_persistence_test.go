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
		baseDir := "/data/hot-restart"

		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("port-forwarding to Hazelcast master pod")
		stopChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")

		By("creating the cache config")
		cache := hazelcastconfig.DefaultCache(mapLookupKey, hazelcast.Name, labels)
		cache.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), cache)).Should(Succeed())
		assertDataStructureStatus(chLookupKey, hazelcastv1alpha1.DataStructureSuccess, cache)

		By("filling the cache with entries")
		entryCount := 100
		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		cli := hz.NewClientInternal(cl)
		fillCacheData(entryCount, cli, cache)

		validateCacheEntries(entryCount, cli, cache)
		closeChannel(stopChan)

		By("creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		assertHotBackupSuccess(hotBackup, 1*Minute)
		seq := GetBackupSequence(t, hzLookupKey)
		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)

		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey, 3)
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1alpha1.RestoreSucceeded)

		By("port-forwarding to restarted Hazelcast master pod")
		stopChan = portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		By("checking the map entries")
		cl = createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			err := cl.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()
		cli = hz.NewClientInternal(cl)

		validateCacheEntries(entryCount, cli, cache)
	})

	It("should persist the cache successfully created configs into the configmap", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hchp-3")
		caches := []string{"cache1", "cache2", "cache3", "cachefail"}

		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

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

		c := hazelcastconfig.DefaultCache(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
		assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastv1alpha1.DataStructureSuccess, c)

		By("checking if the map config is persisted")
		hzConfig := assertCacheConfigsPersisted(hazelcast, c.Name)
		ccfg := hzConfig.Hazelcast.Cache[c.Name]

		By("failing to update the map config")
		c.Spec.BackupCount = pointer.Int32Ptr(4)
		Expect(k8sClient.Update(context.Background(), c)).Should(Succeed())
		assertDataStructureStatus(types.NamespacedName{Name: c.Name, Namespace: c.Namespace}, hazelcastv1alpha1.DataStructureFailed, c)

		By("checking if the same map config is still there")
		// Should wait for Hazelcast reconciler to get triggered, we do not have a waiting mechanism for that.
		Sleep(5 * Second)
		hzConfig = assertCacheConfigsPersisted(hazelcast, c.Name)
		newCcfg := hzConfig.Hazelcast.Cache[c.Name]
		Expect(newCcfg).To(Equal(ccfg))
	})
})

func validateCacheEntries(entryCount int, cli *hz.ClientInternal, cache *hazelcastv1alpha1.Cache) {
	for i := 0; i < entryCount; i++ {
		key, err := cli.EncodeData(fmt.Sprintf("mykey%d", i))
		Expect(err).To(BeNil())
		value := fmt.Sprintf("myvalue%d", i)
		getRequest := codec.EncodeCacheGetRequest("/hz/"+cache.GetDSName(), key, nil)
		resp, err := cli.InvokeOnKey(context.Background(), getRequest, key, nil)
		pairs := codec.DecodeCacheGetResponse(resp)
		Expect(err).To(BeNil())
		data, err := cli.DecodeData(pairs)
		Expect(err).To(BeNil())
		Expect(fmt.Sprintf("%v", data)).Should(Equal(value))
	}
}

func fillCacheData(entryCount int, cli *hz.ClientInternal, cache *hazelcastv1alpha1.Cache) {
	for _, mi := range cli.OrderedMembers() {
		configRequest := codec.EncodeCacheGetConfigRequest("/hz/"+cache.GetDSName(), cache.GetDSName())
		_, _ = cli.InvokeOnMember(context.Background(), configRequest, mi.UUID, nil)
	}

	for i := 0; i < entryCount; i++ {
		key, err := cli.EncodeData(fmt.Sprintf("mykey%d", i))
		Expect(err).To(BeNil())
		value, err := cli.EncodeData(fmt.Sprintf("myvalue%d", i))
		Expect(err).To(BeNil())
		cpr := codec.EncodeCachePutRequest("/hz/"+cache.GetDSName(), key, value, nil, false, 0)
		_, err = cli.InvokeOnKey(context.Background(), cpr, key, nil)
		Expect(err).To(BeNil())
	}
}
