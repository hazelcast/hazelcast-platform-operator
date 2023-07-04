package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("MultiMap CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.MultiMap{}, &hazelcastv1alpha1.MultiMapList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			mm := &hazelcastv1alpha1.MultiMap{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MultiMapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
				},
			}
			By("creating MultiMap CR successfully")
			Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
			mms := mm.Spec

			By("checking the CR values with default ones")
			Expect(mms.Name).To(BeEmpty())
			Expect(*mms.BackupCount).To(Equal(n.DefaultMultiMapBackupCount))
			Expect(mms.Binary).To(Equal(n.DefaultMultiMapBinary))
			Expect(string(mms.CollectionType)).To(Equal(n.DefaultMultiMapCollectionType))
			Expect(mms.HazelcastResourceName).To(Equal("hazelcast"))
		})

		When("applying empty spec", func() {
			It("should fail to create", Label("fast"), func() {
				mm := &hazelcastv1alpha1.MultiMap{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create MultiMap CR")
				Expect(k8sClient.Create(context.Background(), mm)).ShouldNot(Succeed())
			})
		})
	})

	When("applying spec with backupCount and/or asyncBackupCount", func() {
		It("should be successfully with both values under 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.MultiMap{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MultiMapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(2),
						AsyncBackupCount:      2,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		})

		It("should error with backupCount over 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.MultiMap{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MultiMapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(7),
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})

		It("should error with asyncBackupCount over 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.MultiMap{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MultiMapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						AsyncBackupCount:      6,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})

		It("should error with sum of two values over 6", Label("fast"), func() {
			m := &hazelcastv1alpha1.MultiMap{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.MultiMapSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
						BackupCount:           pointer.Int32(5),
						AsyncBackupCount:      5,
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
		})
	})
})
