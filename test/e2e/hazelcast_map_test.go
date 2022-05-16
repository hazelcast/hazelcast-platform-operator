package e2e

import (
	"context"
	"fmt"
	"strconv"
	"time"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Map Config", Label("map"), func() {
	hzName := fmt.Sprintf("hz-map-%d", GinkgoParallelProcess())
	mapName := fmt.Sprintf("map-%d", GinkgoParallelProcess())

	var hzLookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}

	var mapLookupKey = types.NamespacedName{
		Name:      mapName,
		Namespace: hzNamespace,
	}
	labels := map[string]string{
		"test_suite": fmt.Sprintf("map_%d", GinkgoParallelProcess()),
	}
	localPort := strconv.Itoa(8100 + GinkgoParallelProcess())
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("Checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyHazelcast(hzLookupKey), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
		Expect(k8sClient.DeleteAllOf(
			context.Background(), &hazelcastcomv1alpha1.Map{}, client.InNamespace(hzNamespace), client.MatchingLabels(labels))).Should(Succeed())
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
	})

	It("should create Map Config", Label("fast"), func() {
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
	})

	It("should create Map Config with correct default values", Label("fast"), func() {
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("port-forwarding to Hazelcast master pod")
		stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)
		err := waitForReadyChannel(readyChan, 5*time.Second)
		Expect(err).To(BeNil())

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("checking if the map config is created correctly")
		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			err := cl.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()
		mapConfig := getMapConfig(context.Background(), cl, m.MapName())
		Expect(mapConfig.InMemoryFormat).Should(Equal(int32(0)))
		Expect(mapConfig.BackupCount).Should(Equal(n.DefaultMapBackupCount))
		Expect(mapConfig.AsyncBackupCount).Should(Equal(int32(0)))
		Expect(mapConfig.TimeToLiveSeconds).Should(Equal(*m.Spec.TimeToLiveSeconds))
		Expect(mapConfig.MaxIdleSeconds).Should(Equal(*m.Spec.MaxIdleSeconds))
		Expect(mapConfig.MaxSize).Should(Equal(*m.Spec.Eviction.MaxSize))
		Expect(mapConfig.MaxSizePolicy).Should(Equal(hazelcastcomv1alpha1.EncodeMaxSizePolicy[m.Spec.Eviction.MaxSizePolicy]))
		Expect(mapConfig.ReadBackupData).Should(Equal(false))
		Expect(mapConfig.EvictionPolicy).Should(Equal(hazelcastcomv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy]))
		Expect(mapConfig.MergePolicy).Should(Equal("com.hazelcast.spi.merge.PutIfAbsentMergePolicy"))

	})

	It("should create Map Config with Indexes", Label("fast"), func() {
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.BackupCount = pointer.Int32Ptr(3)
		m.Spec.Indexes = []hazelcastcomv1alpha1.IndexConfig{
			{
				Name:               "index-1",
				Type:               hazelcastcomv1alpha1.IndexTypeHash,
				Attributes:         []string{"attribute1", "attribute2"},
				BitmapIndexOptions: nil,
			},
			{
				Name:       "index-2",
				Type:       hazelcastcomv1alpha1.IndexTypeBitmap,
				Attributes: []string{"attribute3", "attribute4"},
				BitmapIndexOptions: &hazelcastcomv1alpha1.BitmapIndexOptionsConfig{
					UniqueKey:           "key",
					UniqueKeyTransition: hazelcastcomv1alpha1.UniqueKeyTransitionRAW,
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("port-forwarding to Hazelcast master pod")
		stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)
		err := waitForReadyChannel(readyChan, 5*time.Second)
		Expect(err).To(BeNil())

		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			err := cl.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()

		By("checking if the map config is created correctly")
		mapConfig := getMapConfig(context.Background(), cl, m.MapName())
		Expect(mapConfig.Indexes[0].Name).Should(Equal("index-1"))
		Expect(mapConfig.Indexes[0].Type).Should(Equal(hazelcastcomv1alpha1.EncodeIndexType[hazelcastcomv1alpha1.IndexTypeHash]))
		Expect(mapConfig.Indexes[0].Attributes).Should(Equal([]string{"attribute1", "attribute2"}))
		// TODO: Hazelcast side returns these bitmapIndexOptions even though we give them empty.
		Expect(mapConfig.Indexes[0].BitmapIndexOptions.UniqueKey).Should(Equal("__key"))
		Expect(mapConfig.Indexes[0].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(int32(0)))

		Expect(mapConfig.Indexes[1].Name).Should(Equal("index-2"))
		Expect(mapConfig.Indexes[1].Type).Should(Equal(hazelcastcomv1alpha1.EncodeIndexType[hazelcastcomv1alpha1.IndexTypeBitmap]))
		Expect(mapConfig.Indexes[1].Attributes).Should(Equal([]string{"attribute3", "attribute4"}))
		Expect(mapConfig.Indexes[1].BitmapIndexOptions.UniqueKey).Should(Equal("key"))
		Expect(mapConfig.Indexes[1].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(hazelcastcomv1alpha1.EncodeUniqueKeyTransition[hazelcastcomv1alpha1.UniqueKeyTransitionRAW]))

	})

	It("should update the map correctly", Label("fast"), func() {
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("port-forwarding to Hazelcast master pod")
		stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)
		err := waitForReadyChannel(readyChan, 5*time.Second)
		Expect(err).To(BeNil())

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("updating the map config")
		m.Spec.TimeToLiveSeconds = pointer.Int32Ptr(150)
		m.Spec.MaxIdleSeconds = pointer.Int32Ptr(100)
		m.Spec.Eviction = &hazelcastcomv1alpha1.EvictionConfig{
			EvictionPolicy: hazelcastcomv1alpha1.EvictionPolicyLFU,
			MaxSize:        pointer.Int32Ptr(500),
			MaxSizePolicy:  hazelcastcomv1alpha1.MaxSizePolicyFreeHeapSize,
		}
		Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("checking if the map config is updated correctly")
		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			err := cl.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()
		mapConfig := getMapConfig(context.Background(), cl, m.MapName())
		Expect(mapConfig.TimeToLiveSeconds).Should(Equal(*m.Spec.TimeToLiveSeconds))
		Expect(mapConfig.MaxIdleSeconds).Should(Equal(*m.Spec.MaxIdleSeconds))
		Expect(mapConfig.ReadBackupData).Should(Equal(false))
		Expect(mapConfig.MaxSize).Should(Equal(*m.Spec.Eviction.MaxSize))
		Expect(mapConfig.MaxSizePolicy).Should(Equal(hazelcastcomv1alpha1.EncodeMaxSizePolicy[m.Spec.Eviction.MaxSizePolicy]))
		Expect(mapConfig.EvictionPolicy).Should(Equal(hazelcastcomv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy]))
	})

	It("should fail to update backupCount", Label("fast"), func() {
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("failing to update map config")
		m.Spec.BackupCount = pointer.Int32Ptr(3)
		Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
	})
})
