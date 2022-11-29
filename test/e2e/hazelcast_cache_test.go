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

var _ = Describe("Hazelcast Cache Config", Label("cache"), func() {
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
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
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
	})

	It("should fail to update Cache Config", Label("fast"), func() {
		setLabelAndCRName("hch-3")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the cache config")
		cs := hazelcastcomv1alpha1.CacheSpec{
			DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
				HazelcastResourceName: hzLookupKey.Name,
				BackupCount:           pointer.Int32(3),
			},
		}
		q := hazelcastconfig.Cache(cs, chLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
		q = assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Cache{}).(*hazelcastcomv1alpha1.Cache)

		By("failing to update cache config")
		q.Spec.BackupCount = pointer.Int32(5)
		q.Spec.AsyncBackupCount = pointer.Int32(20)
		Expect(k8sClient.Update(context.Background(), q)).Should(Succeed())
		assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureFailed, &hazelcastcomv1alpha1.Cache{})
	})
})
