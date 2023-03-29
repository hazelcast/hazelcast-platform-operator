package integration

import (
	"context"
	"encoding/json"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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

	//todo: rename
	Context("Cache CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Cache CR")
				Expect(k8sClient.Create(context.Background(), q)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create Cache CR with default configurations", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.CacheSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
						InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
					},
				}
				By("creating Cache CR successfully")
				Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
				qs := q.Spec

				By("checking the CR values with default ones")
				Expect(qs.Name).To(Equal(""))
				Expect(*qs.BackupCount).To(Equal(n.DefaultCacheBackupCount))
				Expect(qs.AsyncBackupCount).To(Equal(n.DefaultCacheAsyncBackupCount))
				Expect(qs.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(qs.KeyType).To(Equal(""))
				Expect(qs.ValueType).To(Equal(""))
			})

			It("should create Cache CR with native memory configuration", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.CacheSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
						InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
					},
				}
				By("creating Cache CR successfully")
				Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
				qs := q.Spec

				By("checking the CR values with native memory")
				Expect(qs.InMemoryFormat).To(Equal(hazelcastv1alpha1.InMemoryFormatNative))
			})
		})

		It("should fail to update", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues(), ee)

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			test.CheckHazelcastCR(hz, defaultHazelcastSpecValues(), ee)

			cache := cacheOf(hazelcastv1alpha1.CacheSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hz.Name,
					BackupCount:           pointer.Int32(3),
				},
			})

			Expect(k8sClient.Create(context.Background(), cache)).Should(Succeed())

			var err error
			for {
				Expect(k8sClient.Get(
					context.Background(), types.NamespacedName{Namespace: cache.Namespace, Name: cache.Name}, cache)).Should(Succeed())
				cache.Spec.BackupCount = pointer.Int32(5)

				err = k8sClient.Update(context.Background(), cache)
				if errors.IsConflict(err) {
					continue
				}
				break
			}
			Expect(err).Should(MatchError(ContainSubstring("backupCount cannot be updated")))

			deleteResource(lookupKey(cache), cache)
			deleteResource(lookupKey(hz), hz)
		})
	})
})
