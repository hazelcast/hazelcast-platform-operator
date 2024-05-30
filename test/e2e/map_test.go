package e2e

import (
	"context"
	"strconv"
	. "time"

	"k8s.io/apimachinery/pkg/types"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Map - ", Group("map"), func() {
	localPort := strconv.Itoa(8200 + GinkgoParallelProcess())

	configEqualsSpec := func(mapSpec *hazelcastcomv1alpha1.MapSpec) func(config codecTypes.MapConfig) bool {
		return func(config codecTypes.MapConfig) bool {
			return mapSpec.TimeToLiveSeconds == config.TimeToLiveSeconds &&
				mapSpec.MaxIdleSeconds == config.MaxIdleSeconds &&
				!config.ReadBackupData && mapSpec.Eviction.MaxSize == config.MaxSize &&
				config.MaxSizePolicy == hazelcastcomv1alpha1.EncodeMaxSizePolicy[mapSpec.Eviction.MaxSizePolicy] &&
				config.EvictionPolicy == hazelcastcomv1alpha1.EncodeEvictionPolicyType[mapSpec.Eviction.EvictionPolicy]
		}
	}

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, &hazelcastcomv1alpha1.HotBackupList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Creating Map configurations", func() {
		It("creates map config with correct default values", Tag(Kind|Any), func() {
			setLabelAndCRName("hm-1")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			By("creating the map config")
			m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map config is created correctly")
			mapConfig := mapConfigPortForward(context.Background(), hazelcast, localPort, m.MapName())
			Expect(mapConfig.InMemoryFormat).Should(Equal(hazelcastcomv1alpha1.EncodeInMemoryFormat[m.Spec.InMemoryFormat]))
			Expect(mapConfig.BackupCount).Should(Equal(n.DefaultMapBackupCount))
			Expect(mapConfig.AsyncBackupCount).Should(Equal(int32(0)))
			Expect(mapConfig.TimeToLiveSeconds).Should(Equal(m.Spec.TimeToLiveSeconds))
			Expect(mapConfig.MaxIdleSeconds).Should(Equal(m.Spec.MaxIdleSeconds))
			Expect(mapConfig.MaxSize).Should(Equal(m.Spec.Eviction.MaxSize))
			Expect(mapConfig.MaxSizePolicy).Should(Equal(hazelcastcomv1alpha1.EncodeMaxSizePolicy[m.Spec.Eviction.MaxSizePolicy]))
			Expect(mapConfig.ReadBackupData).Should(Equal(false))
			Expect(mapConfig.EvictionPolicy).Should(Equal(hazelcastcomv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy]))
			Expect(mapConfig.MergePolicy).Should(Equal("com.hazelcast.spi.merge.PutIfAbsentMergePolicy"))
		})

		It("creates map config with indexes", Tag(Kind|Any), func() {
			setLabelAndCRName("hm-2")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
			m.Spec.BackupCount = pointer.Int32(3)
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
			stopChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)

			cl := newHazelcastClientPortForward(context.Background(), hazelcast, localPort)
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

		It("persists map config with indexes", Tag(Kind|Any), func() {
			setLabelAndCRName("hm-3")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
			m.Spec.Indexes = []hazelcastcomv1alpha1.IndexConfig{
				{
					Name:       "index-1",
					Type:       hazelcastcomv1alpha1.IndexTypeHash,
					Attributes: []string{"attribute1", "attribute2"},
					BitmapIndexOptions: &hazelcastcomv1alpha1.BitmapIndexOptionsConfig{
						UniqueKey:           "key",
						UniqueKeyTransition: hazelcastcomv1alpha1.UniqueKeyTransitionRAW,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map is in the Config")
			hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)

			By("checking if the indexes are persisted")
			Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Name).Should(Equal("index-1"))
			Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Type).Should(Equal(string(hazelcastcomv1alpha1.IndexTypeHash)))
			Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Attributes).Should(ConsistOf("attribute1", "attribute2"))
			Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].BitmapIndexOptions.UniqueKey).Should(Equal("key"))
			Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(string(hazelcastcomv1alpha1.UniqueKeyTransitionRAW)))
		})

		It("persist and removed map config in/from Hazelcast config", Tag(Kind|EE|AnyCloud), func() {
			setLabelAndCRName("hm-4")
			maps := []string{"map1", "map2", "map3", "mapfail"}

			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating the map configs")
			for i, mapp := range maps {
				m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapp, Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
				m.Spec.Eviction = hazelcastcomv1alpha1.EvictionConfig{MaxSize: int32(i) * 100}
				m.Spec.HazelcastResourceName = hazelcast.Name
				if mapp == "mapfail" {
					m.Spec.HazelcastResourceName = "failedHz"
				}
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				if mapp == "mapfail" {
					assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
					continue
				}
				assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
			}

			By("checking if the maps are in the Config", func() {
				hzConfig := assertMapConfigsPersisted(hazelcast, "map1", "map2", "map3")
				for i, mapp := range maps {
					if mapp != "mapfail" {
						Expect(hzConfig.Hazelcast.Map[mapp].Eviction.Size).Should(Equal(int32(i) * 100))
					}
				}
			})

			By("deleting map2")
			Expect(k8sClient.Delete(context.Background(),
				&hazelcastcomv1alpha1.Map{ObjectMeta: v1.ObjectMeta{Name: "map2", Namespace: hazelcast.Namespace}})).Should(Succeed())

			By("checking if map2 is not persisted in the Config", func() {
				_ = assertMapConfigsPersisted(hazelcast, "map1", "map3")
			})
		})

		It("should persist Map Config with Attributes", Tag(Kind|Any), func() {
			setLabelAndCRName("hm-5")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
			m.Spec.Attributes = []hazelcastcomv1alpha1.AttributeConfig{
				{
					Name:               "attribute1",
					ExtractorClassName: "class1",
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map is in the Config")
			hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)

			By("checking if the indexes are persisted")
			Expect(hzConfig.Hazelcast.Map[m.Name].Attributes[0].Name).Should(Equal("attribute1"))
			Expect(hzConfig.Hazelcast.Map[m.Name].Attributes[0].ExtractorClassName).Should(Equal("class1"))
		})
	})

	Context("Updating map configuration", func() {
		It("updates the map configuration correctly", Tag(Kind|Any), func() {
			setLabelAndCRName("hm-6")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			By("port-forwarding to Hazelcast master pod")
			stopChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)

			By("creating the map config")
			m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("updating the map config")
			m.Spec.TimeToLiveSeconds = 150
			m.Spec.MaxIdleSeconds = 100
			m.Spec.Eviction = hazelcastcomv1alpha1.EvictionConfig{
				EvictionPolicy: hazelcastcomv1alpha1.EvictionPolicyLFU,
				MaxSize:        500,
				MaxSizePolicy:  hazelcastcomv1alpha1.MaxSizePolicyFreeHeapSize,
			}
			Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map config is updated correctly")
			cl := newHazelcastClientPortForward(context.Background(), hazelcast, localPort)
			defer func() {
				err := cl.Shutdown(context.Background())
				Expect(err).To(BeNil())
			}()

			Eventually(func() codecTypes.MapConfig {
				return getMapConfig(context.Background(), cl, m.MapName())
			}, 20*Second, interval).Should(Satisfy(configEqualsSpec(&m.Spec)))

		})

		It("maintains last applied Map Config upon update failure", Label("cluster_scope"), Tag(Kind|Any), func() {
			setLabelAndCRName("hm-7")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("checking if the map config is persisted")
			hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)
			mcfg := hzConfig.Hazelcast.Map[m.Name]

			By("failing to update the map config")
			m.Spec.BackupCount = pointer.Int32(4)
			Expect(k8sClient.Update(context.Background(), m)).ShouldNot(Succeed())

			By("checking if the same map config is still there")
			// Should wait for Hazelcast reconciler to get triggered, we do not have a waiting mechanism for that.
			Sleep(5 * Second)
			hzConfig = assertMapConfigsPersisted(hazelcast, m.Name)
			newMcfg := hzConfig.Hazelcast.Map[m.Name]
			Expect(newMcfg).To(Equal(mcfg))
		})
	})

	Context("Validating map configurations", func() {
		When("native memory is not enabled for Hazelcast CR", func() {
			It("fails creating a map configuration with InMemoryFormat value", Tag(Kind|Any), func() {
				setLabelAndCRName("hm-8")
				hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
				CreateHazelcastCR(hazelcast)

				By("creating the map config with NativeMemory")
				m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
				m.Spec.InMemoryFormat = hazelcastcomv1alpha1.InMemoryFormatNative

				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				m = assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
				Expect(m.Status.Message).To(ContainSubstring("Native Memory must be enabled at Hazelcast"))
			})

			It("fails to set InMemoryFormat as NativeMemory in near cache", Tag(Kind|Any), func() {
				setLabelAndCRName("hm-9")
				hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
				CreateHazelcastCR(hazelcast)

				By("creating the map config with NativeMemory enabled for near cache")
				m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
				m.Spec.NearCache = &hazelcastcomv1alpha1.NearCache{InMemoryFormat: hazelcastcomv1alpha1.InMemoryFormatNative}

				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				m = assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
				Expect(m.Status.Message).To(ContainSubstring("Native Memory must be enabled at Hazelcast"))
			})
		})

		It("fails when map CR persistence setting mismatches Hazelcast CR setting", Tag(Kind|Any), func() {
			setLabelAndCRName("hm-10")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)

			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			m = assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
			Expect(m.Status.Message).To(ContainSubstring("Persistence must be enabled at Hazelcast"))
		})

	})

})
