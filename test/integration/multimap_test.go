package integration

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MultiMap CR", func() {
	const namespace = "default"

	//todo: rename
	Context("MultiMap CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				mm := &hazelcastv1alpha1.MultiMap{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create MultiMap CR")
				Expect(k8sClient.Create(context.Background(), mm)).ShouldNot(Succeed())
			})
		})
		When("Using default configuration", func() {
			It("should create MultiMap CR with default configurations", Label("fast"), func() {
				mm := &hazelcastv1alpha1.MultiMap{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.MultiMapSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
					},
				}
				By("creating MultiMap CR successfully")
				Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
				mms := mm.Spec

				By("checking the CR values with default ones")
				Expect(mms.Name).To(Equal(""))
				Expect(*mms.BackupCount).To(Equal(n.DefaultMultiMapBackupCount))
				Expect(mms.Binary).To(Equal(n.DefaultMultiMapBinary))
				Expect(string(mms.CollectionType)).To(Equal(n.DefaultMultiMapCollectionType))
				Expect(mms.HazelcastResourceName).To(Equal("hazelcast"))
			})
		})
	})
})
