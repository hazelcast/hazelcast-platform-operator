package integration

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Topic CR", func() {
	const namespace = "default"

	//todo: rename
	Context("Topic CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				t := &hazelcastv1alpha1.Topic{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Topic CR")
				Expect(k8sClient.Create(context.Background(), t)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create Topic CR with default configurations", Label("fast"), func() {
				t := &hazelcastv1alpha1.Topic{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.TopicSpec{
						HazelcastResourceName: "hazelcast",
					},
				}
				By("creating Topic CR successfully")
				Expect(k8sClient.Create(context.Background(), t)).Should(Succeed())
				ts := t.Spec

				By("checking the CR values with default ones")
				Expect(ts.Name).To(Equal(""))
				Expect(ts.GlobalOrderingEnabled).To(Equal(n.DefaultTopicGlobalOrderingEnabled))
				Expect(ts.MultiThreadingEnabled).To(Equal(n.DefaultTopicMultiThreadingEnabled))
				Expect(ts.HazelcastResourceName).To(Equal("hazelcast"))
			})
		})
	})
})
