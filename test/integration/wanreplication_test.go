package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

var _ = Describe("WanReplication CR", func() {
	const namespace = "default"

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			wan := &hazelcastv1alpha1.WanReplication{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.WanReplicationSpec{
					Resources: []hazelcastv1alpha1.ResourceSpec{
						{
							Name: "hazelcast",
							Kind: hazelcastv1alpha1.ResourceKindHZ,
						},
					},
					TargetClusterName: "dev",
					Endpoints:         "10.0.0.1:5701",
				},
			}
			By("creating WanReplication CR successfully")
			Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
			ws := wan.Spec

			By("checking the CR values with default ones")
			Expect(ws.Resources).To(HaveLen(1))
			Expect(ws.Resources[0].Kind).To(Equal(hazelcastv1alpha1.ResourceKindHZ))
			Expect(ws.Resources[0].Name).To(Equal("hazelcast"))
			Expect(ws.TargetClusterName).To(Equal("dev"))
			Expect(ws.Endpoints).To(Equal("10.0.0.1:5701"))
		})

		When("applying empty spec", func() {
			It("should fail to create", Label("fast"), func() {
				wan := &hazelcastv1alpha1.WanReplication{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Cache CR")
				Expect(k8sClient.Create(context.Background(), wan)).ShouldNot(Succeed())
			})
		})
	})

	Context("with Endpoints value", func() {
		When("endpoints are configured without port", func() {
			It("should set default port to endpoints", Label("fast"), func() {
				endpoints := "10.0.0.1,10.0.0.2,10.0.0.3"
				wan := &hazelcastv1alpha1.WanReplication{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.WanReplicationSpec{
						Resources: []hazelcastv1alpha1.ResourceSpec{
							{
								Name: "hazelcast",
								Kind: hazelcastv1alpha1.ResourceKindHZ,
							},
						},
						TargetClusterName: "dev",
						Endpoints:         endpoints,
					},
				}
				By("creating WanReplication CR successfully")
				Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
				Eventually(func() string {
					newWan := hazelcastv1alpha1.WanReplication{}
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: wan.Name, Namespace: wan.Namespace}, &newWan)
					if err != nil {
						return ""
					}
					return newWan.Spec.Endpoints
				}, timeout, interval).Should(Equal("10.0.0.1:5710,10.0.0.2:5710,10.0.0.3:5710"))
			})
		})
	})
})
