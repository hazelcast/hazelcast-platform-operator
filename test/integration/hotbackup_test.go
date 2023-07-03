package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("HotBackup CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.HotBackup{}, &hazelcastv1alpha1.HotBackupList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

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
		})
	})
})
