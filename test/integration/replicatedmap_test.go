package integration

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReplicatedMap CR", func() {
	const namespace = "default"

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			rm := &hazelcastv1alpha1.ReplicatedMap{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.ReplicatedMapSpec{
					HazelcastResourceName: "hazelcast",
				},
			}
			By("creating ReplicatedMap CR successfully")
			Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
			rms := rm.Spec

			By("checking the CR values with default ones")
			Expect(rms.Name).To(Equal(""))
			Expect(string(rms.InMemoryFormat)).To(Equal(n.DefaultReplicatedMapInMemoryFormat))
			Expect(*rms.AsyncFillup).To(Equal(n.DefaultReplicatedMapAsyncFillup))
			Expect(rms.HazelcastResourceName).To(Equal("hazelcast"))
		})

		When("using empty spec", func() {
			It("should fail to create", Label("fast"), func() {
				t := &hazelcastv1alpha1.ReplicatedMap{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create ReplicatedMap CR")
				Expect(k8sClient.Create(context.Background(), t)).ShouldNot(Succeed())
			})
		})
	})

})
