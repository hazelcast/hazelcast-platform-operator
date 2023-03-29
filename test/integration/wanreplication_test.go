package integration

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("WanReplication CR", func() {
	const namespace = "default"

	//todo: rename
	Context("WanReplication CR configuration", func() {
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
				}, timeout, interval).Should(Equal("10.0.0.1:5701,10.0.0.2:5701,10.0.0.3:5701"))
			})
		})
	})
})
