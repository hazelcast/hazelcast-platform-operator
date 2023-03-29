package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Webhook", func() {
	const (
		namespace = "default"
	)
	repository := n.HazelcastRepo
	if ee {
		repository = n.HazelcastEERepo
	}

	defaultSpecValues := &test.HazelcastSpecValues{
		ClusterSize:     n.DefaultClusterSize,
		Repository:      repository,
		Version:         n.HazelcastVersion,
		LicenseKey:      n.LicenseKeySecret,
		ImagePullPolicy: n.HazelcastImagePullPolicy,
	}

	GetRandomObjectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("hazelcast-test-%s", uuid.NewUUID()),
			Namespace: namespace,
		}
	}

	Cache := func(cacheSpec hazelcastv1alpha1.CacheSpec) *hazelcastv1alpha1.Cache {
		cs, _ := json.Marshal(cacheSpec)
		return &hazelcastv1alpha1.Cache{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("cache-test-%s", uuid.NewUUID()),
				Namespace: namespace,
				Annotations: map[string]string{
					n.LastSuccessfulSpecAnnotation: string(cs),
				},
			},
			Spec: cacheSpec,
		}
	}

	Map := func(mapSpec hazelcastv1alpha1.MapSpec) *hazelcastv1alpha1.Map {
		ms, _ := json.Marshal(mapSpec)
		return &hazelcastv1alpha1.Map{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("map-test-%s", uuid.NewUUID()),
				Namespace: namespace,
				Annotations: map[string]string{
					n.LastSuccessfulSpecAnnotation: string(ms),
				},
			},
			Spec: mapSpec,
		}
	}

	Delete := func(obj client.Object) {
		By("expecting to delete CR successfully")
		deleteIfExists(lookupKey(obj), obj)
		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey(obj), obj)
	}

	Context("Cache Validation", func() {
		It("should fail to update", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			test.CheckHazelcastCR(hz, defaultSpecValues, ee)

			cache := Cache(hazelcastv1alpha1.CacheSpec{
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

			Delete(cache)
			Delete(hz)
		})
	})

	Context("Map Validation", func() {
		It("should fail to update", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			test.CheckHazelcastCR(hz, defaultSpecValues, ee)

			m := Map(hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hz.Name,
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
			Expect(err).Should(MatchError(ContainSubstring("backupCount cannot be updated")))

			Delete(m)
			Delete(hz)
		})
	})
})
