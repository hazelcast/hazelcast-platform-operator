package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
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

	Context("HotBackup is deleted", func() {
		It("should not be allowed when it is referenced by Hazelcast restore", Label("fast"), func() {
			hb := &hazelcastv1alpha1.HotBackup{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.HotBackupSpec{
					HazelcastResourceName: "hazelcast",
				},
			}
			By("creating HotBackup CR successfully")
			Expect(k8sClient.Create(context.Background(), hb)).Should(Succeed())

			spec := test.HazelcastSpec(defaultHazelcastSpecValues(), ee)
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir: "/baseDir/",
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
				Restore: hazelcastv1alpha1.RestoreConfiguration{
					HotBackupResourceName: hb.Name,
				},
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			By("creating the Hazelcast CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

			By("trying to delete the HotBackup resource which is referenced by Hazelcast restore")
			err := k8sClient.Delete(context.Background(), hb)
			Expect(err).Should(MatchError(ContainSubstring(fmt.Sprintf("Hazelcast '%s' has a restore reference to the Hotbackup", hz.Name))))

			By("deleting the Hazelcast CR")
			hz.Finalizers = make([]string, 0) // It is required to be able to delete the Hazelcast resource
			Expect(k8sClient.Delete(context.Background(), hz)).Should(Succeed())
			assertDoesNotExist(lookupKey(hz), hz)

			By("deleting the HotBackup CR which is not referenced by Hazelcast restore")
			Expect(k8sClient.Delete(context.Background(), hb)).Should(Succeed())
		})
	})
})
