package integration

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var _ = Describe("JetJobSnapshot CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	Context("JetJobSnapshot create validation", func() {
		It("should let create JetJobSnapshot with empty snapshot name", Label("fast"), func() {
			jjs := &hazelcastv1alpha1.JetJobSnapshot{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.JetJobSnapshotSpec{
					Name:               "",
					JetJobResourceName: "jetjobname",
					CancelJob:          false,
				},
			}

			Expect(k8sClient.Create(context.Background(), jjs)).Should(Not(HaveOccurred()))
		})

		It("should not create JetJobSnapshot with empty jetJobResourceName", Label("fast"), func() {
			jjs := &hazelcastv1alpha1.JetJobSnapshot{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.JetJobSnapshotSpec{
					Name:               "snapshot-1",
					JetJobResourceName: "",
					CancelJob:          false,
				},
			}

			Expect(k8sClient.Create(context.Background(), jjs)).
				Should(MatchError(ContainSubstring("jetJobResourceName cannot be empty")))
		})
	})

	Context("JetJobSnapshot update validation", func() {
		It("should not update immutable fields", Label("fast"), func() {
			spec := hazelcastv1alpha1.JetJobSnapshotSpec{
				Name:               "snapshot-1",
				JetJobResourceName: "jetjobname",
				CancelJob:          false,
			}
			js, _ := json.Marshal(spec)

			jjs := &hazelcastv1alpha1.JetJobSnapshot{
				ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(js)),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), jjs)).Should(Not(HaveOccurred()))

			updatedJjs := jjs.DeepCopy()
			updatedJjs.Spec.Name = "snapshot-2"
			Expect(k8sClient.Update(context.Background(), updatedJjs)).
				Should(MatchError(ContainSubstring("field cannot be updated")))

			updatedJjs = jjs.DeepCopy()
			updatedJjs.Spec.JetJobResourceName = "newjetjobname"
			Expect(k8sClient.Update(context.Background(), updatedJjs)).
				Should(MatchError(ContainSubstring("field cannot be updated")))

			updatedJjs = jjs.DeepCopy()
			updatedJjs.Spec.CancelJob = true
			Expect(k8sClient.Update(context.Background(), updatedJjs)).
				Should(MatchError(ContainSubstring("field cannot be updated")))
		})
	})
})
