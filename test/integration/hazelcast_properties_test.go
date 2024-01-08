package integration

import (
	"context"
	"fmt"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hazelcast Properties", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with custom properties", func() {
		It("should add new property", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.Properties = make(map[string]string)
			hz.Spec.Properties["hazelcast.graceful.shutdown.max.wait"] = "300"

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			ensureHzStatusIsPending(hz)

			fetchedCR := fetchHz(hz)
			Expect(fetchedCR.Spec.Properties["hazelcast.cluster.version.auto.upgrade.enabled"]).Should(Equal("true"))
			Expect(fetchedCR.Spec.Properties["hazelcast.graceful.shutdown.max.wait"]).Should(Equal("300"))
		})

		It("should not override default property", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.Properties = make(map[string]string)
			hz.Spec.Properties["hazelcast.cluster.version.auto.upgrade.enabled"] = "false"

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			ensureHzStatusIsPending(hz)

			fetchedCR := fetchHz(hz)
			Expect(fetchedCR.Spec.Properties["hazelcast.cluster.version.auto.upgrade.enabled"]).Should(Equal("true"))
		})
	})
})
