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

var _ = Describe("Cache CR", func() {
	const namespace = "default"

	cacheOf := func(cacheSpec hazelcastv1alpha1.CacheSpec) *hazelcastv1alpha1.Cache {
		cs, _ := json.Marshal(cacheSpec)
		return &hazelcastv1alpha1.Cache{
			ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(cs)),
			Spec:       cacheSpec,
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
		DeleteAllOf(&hazelcastv1alpha1.Cache{}, &hazelcastv1alpha1.CacheList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			c := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
				},
			}
			By("creating Cache CR successfully")
			Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
			cs := c.Spec

			By("checking the CR values with default ones")
			Expect(cs.Name).To(BeEmpty())
			Expect(*cs.BackupCount).To(Equal(n.DefaultCacheBackupCount))
			Expect(cs.AsyncBackupCount).To(Equal(n.DefaultCacheAsyncBackupCount))
			Expect(cs.HazelcastResourceName).To(Equal("hazelcast"))
			Expect(cs.KeyType).To(BeEmpty())
			Expect(cs.ValueType).To(BeEmpty())
			Expect(cs.InMemoryFormat).To(Equal(hazelcastv1alpha1.InMemoryFormatBinary))
			Expect(cs.EventJournal).To(BeNil())
		})

		When("applying empty spec", func() {
			It("should fail to create", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Cache CR")
				Expect(k8sClient.Create(context.Background(), q)).ShouldNot(Succeed())
			})
		})
	})

	Context("with BackupCount value", func() {
		When("updating BackupCount", func() {
			It("should fail to update", Label("fast"), func() {
				cache := cacheOf(hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(3),
					},
				})

				Expect(k8sClient.Create(context.Background(), cache)).Should(Succeed())

				var err error
				for {
					Expect(k8sClient.Get(context.Background(), lookupKey(cache), cache)).Should(Succeed())
					cache.Spec.BackupCount = pointer.Int32(5)

					err = k8sClient.Update(context.Background(), cache)
					if errors.IsConflict(err) {
						continue
					}
					break
				}

				Expect(err).Should(MatchError(ContainSubstring("spec: Forbidden: cannot be updated")))
			})
		})
	})

	Context("with InMemoryFormat value", func() {
		It("should create successfully with NativeMemory", Label("fast"), func() {
			c := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
					InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
				},
			}
			By("creating Cache CR successfully")
			Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
			cs := c.Spec

			By("checking the CR value with native memory")
			Expect(cs.InMemoryFormat).To(Equal(hazelcastv1alpha1.InMemoryFormatNative))
		})
	})

	When("applying spec with backupCount and/or asyncBackupCount", func() {
		It("should be successfully with both values under 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(2),
						AsyncBackupCount:      2,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		})

		It("should error with backupCount over 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(7),
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})

		It("should error with asyncBackupCount over 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						AsyncBackupCount:      6,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})

		It("should error with sum of two values over 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
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

	Context("with EventJournal configuration", func() {
		It("should create Cache CR with eventJournal configuration", Label("fast"), func() {
			c := &hazelcastv1alpha1.Cache{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.CacheSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
					EventJournal: &hazelcastv1alpha1.EventJournal{
						Capacity:          10000,
						TimeToLiveSeconds: 30,
					},
				},
			}

			By("creating Cache CR successfully")
			Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())
			cs := c.Spec

			By("checking the Cache CR values")
			Expect(cs.EventJournal).ToNot(BeNil())
			Expect(cs.EventJournal.Capacity).To(Equal(int32(10000)))
			Expect(cs.EventJournal.TimeToLiveSeconds).To(Equal(int32(30)))
		})

		It("should fail to update", Label("fast"), func() {
			c := cacheOf(hazelcastv1alpha1.CacheSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: "hazelcast",
				},
				EventJournal: &hazelcastv1alpha1.EventJournal{
					Capacity:          10000,
					TimeToLiveSeconds: 30,
				},
			})

			By("creating Cache CR successfully")
			Expect(k8sClient.Create(context.Background(), c)).Should(Succeed())

			By("trying to update eventJournal")
			var err error
			for {
				Expect(k8sClient.Get(
					context.Background(), types.NamespacedName{Namespace: c.Namespace, Name: c.Name}, c)).Should(Succeed())

				c.Spec.EventJournal.Capacity = 9000
				c.Spec.EventJournal.TimeToLiveSeconds = 0

				err = k8sClient.Update(context.Background(), c)
				if errors.IsConflict(err) {
					continue
				}
				break
			}

			Expect(err).Should(MatchError(ContainSubstring("Forbidden: cannot be updated")))
		})
	})
})
