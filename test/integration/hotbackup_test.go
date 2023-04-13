package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

var _ = Describe("HotBackup CR", func() {
	const namespace = "default"

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			hb := &hazelcastv1alpha1.HotBackup{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.HotBackupSpec{
					HazelcastResourceName: "hazelcast",
				},
			}
			By("creating HotBackup CR successfully")
			Expect(k8sClient.Create(context.Background(), hb)).Should(Succeed())

			By("checking the CR values with default ones")
			Expect(hb.Spec.HazelcastResourceName).To(Equal("hazelcast"))

			deleteResource(lookupKey(hb), hb)
		})
	})
})
