package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("ReplicatedMap CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.ReplicatedMap{}, &hazelcastv1alpha1.ReplicatedMapList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

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
			Expect(rms.Name).To(BeEmpty())
			Expect(string(rms.InMemoryFormat)).To(Equal(n.DefaultReplicatedMapInMemoryFormat))
			Expect(*rms.AsyncFillup).To(Equal(n.DefaultReplicatedMapAsyncFillup))
			Expect(rms.HazelcastResourceName).To(Equal("hazelcast"))
		})

		When("applying empty spec", func() {
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
