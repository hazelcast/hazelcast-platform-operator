package integration

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("Map CR", func() {
	const namespace = "default"

	mapOf := func(mapSpec hazelcastv1alpha1.MapSpec) *hazelcastv1alpha1.Map {
		ms, _ := json.Marshal(mapSpec)
		return &hazelcastv1alpha1.Map{
			ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(ms)),
			Spec:       mapSpec,
		}
	}

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Map{}, &hazelcastv1alpha1.MapList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create successfully", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
				},
			}
			By("creating Map CR successfully")
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			ms := m.Spec

			By("checking the CR values with default ones")
			Expect(ms.Name).To(BeEmpty())
			Expect(*ms.BackupCount).To(Equal(n.DefaultMapBackupCount))
			Expect(ms.TimeToLiveSeconds).To(Equal(n.DefaultMapTimeToLiveSeconds))
			Expect(ms.MaxIdleSeconds).To(Equal(n.DefaultMapMaxIdleSeconds))
			Expect(ms.Eviction.EvictionPolicy).To(Equal(hazelcastv1alpha1.EvictionPolicyType(n.DefaultMapEvictionPolicy)))
			Expect(ms.Eviction.MaxSize).To(Equal(n.DefaultMapMaxSize))
			Expect(ms.Eviction.MaxSizePolicy).To(Equal(hazelcastv1alpha1.MaxSizePolicyType(n.DefaultMapMaxSizePolicy)))
			Expect(ms.Indexes).To(BeNil())
			Expect(ms.Attributes).To(BeNil())
			Expect(ms.PersistenceEnabled).To(Equal(n.DefaultMapPersistenceEnabled))
			Expect(ms.HazelcastResourceName).To(Equal("hazelcast"))
			Expect(ms.EntryListeners).To(BeNil())
			Expect(ms.EventJournal).To(BeNil())
		})

		When("applying empty spec", func() {
			It("should fail to create", func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Map CR")
				Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
				assertDoesNotExist(lookupKey(m), m)
			})
		})
	})

	Context("with BackupCount value", func() {
		When("updating BackupCount", func() {
			It("should fail to update", func() {
				m := mapOf(hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(3),
					},
				})

				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())

				var err error
				for {
					Expect(k8sClient.Get(
						context.Background(), types.NamespacedName{Namespace: m.Namespace, Name: m.Name}, m)).Should(Succeed())
					m.Spec.BackupCount = pointer.Int32(5)

					err = k8sClient.Update(context.Background(), m)
					if errors.IsConflict(err) {
						continue
					}
					break
				}

				Expect(err).Should(MatchError(ContainSubstring("spec.backupCount: Forbidden: field cannot be updated")))
			})
		})
	})

	When("applying spec with backupCount and/or asyncBackupCount", func() {
		It("should be successfully with both values under 6", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(2),
						AsyncBackupCount:      2,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		})

		It("should error with backupCount over 6", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(7),
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})

		It("should error with asyncBackupCount over 6", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						AsyncBackupCount:      6,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})

		It("should error with sum of two values over 6", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(5),
						AsyncBackupCount:      5,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})
	})

	Context("with InMemoryFormat value", func() {
		It("should create successfully with NativeMemory", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
					InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				},
			}
			By("creating Map CR successfully")
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			ms := m.Spec

			By("checking the CR values with native memory")
			Expect(ms.InMemoryFormat).To(Equal(hazelcastv1alpha1.InMemoryFormatNative))
		})
	})

	Context("with NearCache configuration", func() {
		It("should not fail with empty near cache configuration", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
					NearCache: &hazelcastv1alpha1.NearCache{},
				},
			}
			By("creating Map CR successfully")
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		})

		It("should create Map CR with near cache configuration", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
					NearCache: &hazelcastv1alpha1.NearCache{
						Name:               "mostly-used-map",
						InMemoryFormat:     "OBJECT",
						InvalidateOnChange: pointer.Bool(false),
						TimeToLiveSeconds:  300,
						MaxIdleSeconds:     300,
						NearCacheEviction: hazelcastv1alpha1.NearCacheEviction{
							EvictionPolicy: "NONE",
							MaxSizePolicy:  "ENTRY_COUNT",
							Size:           10,
						},
						CacheLocalEntries: pointer.Bool(false),
					},
				},
			}
			By("creating Map CR successfully")
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			ms := m.Spec

			By("checking the CR values with default ones")
			Expect(ms.Name).To(BeEmpty())
			Expect(*ms.BackupCount).To(Equal(n.DefaultMapBackupCount))
			Expect(ms.TimeToLiveSeconds).To(Equal(n.DefaultMapTimeToLiveSeconds))
			Expect(ms.MaxIdleSeconds).To(Equal(n.DefaultMapMaxIdleSeconds))
			Expect(ms.Eviction.EvictionPolicy).To(Equal(hazelcastv1alpha1.EvictionPolicyType(n.DefaultMapEvictionPolicy)))
			Expect(ms.Eviction.MaxSize).To(Equal(n.DefaultMapMaxSize))
			Expect(ms.Eviction.MaxSizePolicy).To(Equal(hazelcastv1alpha1.MaxSizePolicyType(n.DefaultMapMaxSizePolicy)))
			Expect(ms.Indexes).To(BeNil())
			Expect(ms.Attributes).To(BeNil())
			Expect(ms.PersistenceEnabled).To(Equal(n.DefaultMapPersistenceEnabled))
			Expect(ms.HazelcastResourceName).To(Equal("hazelcast"))
			Expect(ms.EntryListeners).To(BeNil())
			Expect(ms.NearCache.Name).To(Equal(m.Spec.NearCache.Name))
			Expect(ms.NearCache.CacheLocalEntries).To(Equal(m.Spec.NearCache.CacheLocalEntries))
			Expect(ms.NearCache.InvalidateOnChange).To(Equal(m.Spec.NearCache.InvalidateOnChange))
			Expect(ms.NearCache.MaxIdleSeconds).To(Equal(m.Spec.NearCache.MaxIdleSeconds))
			Expect(ms.NearCache.InMemoryFormat).To(Equal(m.Spec.NearCache.InMemoryFormat))
			Expect(ms.NearCache.TimeToLiveSeconds).To(Equal(m.Spec.NearCache.TimeToLiveSeconds))
			Expect(ms.NearCache.NearCacheEviction.EvictionPolicy).To(Equal(m.Spec.NearCache.NearCacheEviction.EvictionPolicy))
			Expect(ms.NearCache.NearCacheEviction.Size).To(Equal(m.Spec.NearCache.NearCacheEviction.Size))
			Expect(ms.NearCache.NearCacheEviction.MaxSizePolicy).To(Equal(m.Spec.NearCache.NearCacheEviction.MaxSizePolicy))
		})
	})

	Context("with EventJournal configuration", func() {
		It("should create Map CR with eventJournal configuration", func() {
			m := &hazelcastv1alpha1.Map{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
					EventJournal: &hazelcastv1alpha1.EventJournal{
						Capacity:          10000,
						TimeToLiveSeconds: 30,
					},
				},
			}

			By("creating Map CR successfully")
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			ms := m.Spec

			By("checking the Map CR values")
			Expect(ms.EventJournal).ToNot(BeNil())
			Expect(ms.EventJournal.Capacity).To(Equal(int32(10000)))
			Expect(ms.EventJournal.TimeToLiveSeconds).To(Equal(int32(30)))
		})

		It("should fail to update", func() {
			m := mapOf(hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: "hazelcast",
					BackupCount:           pointer.Int32(3),
				},
				EventJournal: &hazelcastv1alpha1.EventJournal{
					Capacity:          10000,
					TimeToLiveSeconds: 30,
				},
			})

			By("creating Map CR successfully")
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())

			By("trying to update eventJournal")
			var err error
			for {
				Expect(k8sClient.Get(
					context.Background(), types.NamespacedName{Namespace: m.Namespace, Name: m.Name}, m)).Should(Succeed())

				m.Spec.EventJournal.Capacity = 9000
				m.Spec.EventJournal.TimeToLiveSeconds = 0

				err = k8sClient.Update(context.Background(), m)
				if errors.IsConflict(err) {
					continue
				}
				break
			}

			Expect(err).Should(MatchError(ContainSubstring("field cannot be updated")))
		})
	})
})
