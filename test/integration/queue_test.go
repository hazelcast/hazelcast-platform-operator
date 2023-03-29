package integration

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Queue CR", func() {
	const namespace = "default"

	//todo: rename
	Context("Queue CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				q := &hazelcastv1alpha1.Queue{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Queue CR")
				Expect(k8sClient.Create(context.Background(), q)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create Queue CR with default configurations", Label("fast"), func() {
				q := &hazelcastv1alpha1.Queue{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.QueueSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
					},
				}
				By("creating Queue CR successfully")
				Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
				qs := q.Spec

				By("checking the CR values with default ones")
				Expect(qs.Name).To(Equal(""))
				Expect(*qs.BackupCount).To(Equal(n.DefaultQueueBackupCount))
				Expect(qs.AsyncBackupCount).To(Equal(n.DefaultQueueAsyncBackupCount))
				Expect(qs.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(*qs.EmptyQueueTtlSeconds).To(Equal(n.DefaultQueueEmptyQueueTtl))
				Expect(qs.MaxSize).To(Equal(n.DefaultMapMaxSize))
				Expect(qs.PriorityComparatorClassName).To(Equal(""))
			})
		})
	})

})
