package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var _ = Describe("Map CR", func() {
	const namespace = "default"

	mapOf := func(mapSpec hazelcastv1alpha1.MapSpec) *hazelcastv1alpha1.Map {
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

	//todo: rename
	Context("Map CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Map CR")
				Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
				assertDoesNotExist(lookupKey(m), m)
			})
		})

		When("Using default configuration", func() {
			It("should create Map CR with default configurations", Label("fast"), func() {
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
				Expect(ms.Name).To(Equal(""))
				Expect(*ms.BackupCount).To(Equal(n.DefaultMapBackupCount))
				Expect(ms.TimeToLiveSeconds).To(Equal(n.DefaultMapTimeToLiveSeconds))
				Expect(ms.MaxIdleSeconds).To(Equal(n.DefaultMapMaxIdleSeconds))
				Expect(ms.Eviction.EvictionPolicy).To(Equal(hazelcastv1alpha1.EvictionPolicyType(n.DefaultMapEvictionPolicy)))
				Expect(ms.Eviction.MaxSize).To(Equal(n.DefaultMapMaxSize))
				Expect(ms.Eviction.MaxSizePolicy).To(Equal(hazelcastv1alpha1.MaxSizePolicyType(n.DefaultMapMaxSizePolicy)))
				Expect(ms.Indexes).To(BeNil())
				Expect(ms.PersistenceEnabled).To(Equal(n.DefaultMapPersistenceEnabled))
				Expect(ms.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(ms.EntryListeners).To(BeNil())
				deleteResource(lookupKey(m), m)
			})

			It("should create Map CR with native memory configuration", Label("fast"), func() {
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
				deleteResource(lookupKey(m), m)
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

			m := mapOf(hazelcastv1alpha1.MapSpec{
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

			deleteResource(lookupKey(m), m)
			deleteResource(lookupKey(hz), hz)
		})
	})
})
