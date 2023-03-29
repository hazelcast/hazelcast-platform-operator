package integration

import (
	"context"
	"fmt"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var _ = Describe("ReplicatedMap CR", func() {
	const (
		namespace = "default"
	)

	GetRandomObjectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("hazelcast-test-%s", uuid.NewUUID()),
			Namespace: namespace,
		}
	}

	//todo: rename
	Context("ReplicatedMap CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				t := &hazelcastv1alpha1.ReplicatedMap{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create ReplicatedMap CR")
				Expect(k8sClient.Create(context.Background(), t)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create ReplicatedMap CR with default configurations", Label("fast"), func() {
				rm := &hazelcastv1alpha1.ReplicatedMap{
					ObjectMeta: GetRandomObjectMeta(),
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
		})
	})

})
