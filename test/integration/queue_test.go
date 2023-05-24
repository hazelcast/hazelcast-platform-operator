package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("Queue CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	Context("with default configuration", func() {
		It("should create successfully", Label("fast"), func() {
			q := &hazelcastv1alpha1.Queue{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.QueueSpec{
					DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
						HazelcastResourceName: "hazelcast",
					},
				},
			}
			By("creating Queue CR successfully")
			Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
			qs := q.Spec

			By("checking the CR values with default ones")
			Expect(qs.Name).To(BeEmpty())
			Expect(*qs.BackupCount).To(Equal(n.DefaultQueueBackupCount))
			Expect(qs.AsyncBackupCount).To(Equal(n.DefaultQueueAsyncBackupCount))
			Expect(qs.HazelcastResourceName).To(Equal("hazelcast"))
			Expect(*qs.EmptyQueueTtlSeconds).To(Equal(n.DefaultQueueEmptyQueueTtl))
			Expect(qs.MaxSize).To(Equal(n.DefaultMapMaxSize))
			Expect(qs.PriorityComparatorClassName).To(BeEmpty())
		})

		When("applying empty spec", func() {
			It("should fail to create", Label("fast"), func() {
				q := &hazelcastv1alpha1.Queue{
					ObjectMeta: randomObjectMeta(namespace),
				}
				By("failing to create Queue CR")
				Expect(k8sClient.Create(context.Background(), q)).ShouldNot(Succeed())
			})
		})
	})

})
