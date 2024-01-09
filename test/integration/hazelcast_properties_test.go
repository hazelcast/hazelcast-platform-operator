package integration

import (
	"context"
	"fmt"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
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
			hz.Spec.Properties = map[string]string{
				"hazelcast.graceful.shutdown.max.wait": "300",
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			ensureHzStatusIsPending(hz)

			Eventually(func() map[string]string {
				cfg := getSecret(hz)
				a := &config.HazelcastWrapper{}

				if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
					return nil
				}

				return a.Hazelcast.Properties
			}, timeout, interval).Should(Equal(map[string]string{
				"hazelcast.cluster.version.auto.upgrade.enabled": "true",
				"hazelcast.graceful.shutdown.max.wait":           "300",
			}))
		})

		It("should not override default property", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.Properties = map[string]string{
				"hazelcast.cluster.version.auto.upgrade.enabled": "false",
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			ensureHzStatusIsPending(hz)

			Eventually(func() map[string]string {
				cfg := getSecret(hz)
				a := &config.HazelcastWrapper{}

				if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
					return nil
				}

				return a.Hazelcast.Properties
			}, timeout, interval).Should(Equal(map[string]string{
				"hazelcast.cluster.version.auto.upgrade.enabled": "true",
			}))
		})
	})
})
