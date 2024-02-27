package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("Topic CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Topic{}, &hazelcastv1alpha1.TopicList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create successfully", func() {
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
			Expect(ts.Name).To(BeEmpty())
			Expect(ts.GlobalOrderingEnabled).To(Equal(n.DefaultTopicGlobalOrderingEnabled))
			Expect(ts.MultiThreadingEnabled).To(Equal(n.DefaultTopicMultiThreadingEnabled))
			Expect(ts.HazelcastResourceName).To(Equal("hazelcast"))
		})

		When("applying empty spec", func() {
			It("should fail to create", func() {
				t := &hazelcastv1alpha1.Topic{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Topic CR")
				Expect(k8sClient.Create(context.Background(), t)).ShouldNot(Succeed())
			})
		})
	})
})
