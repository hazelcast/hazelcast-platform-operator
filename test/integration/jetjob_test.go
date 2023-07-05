package integration

import (
	"context"
	"encoding/json"
	"fmt"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("JetJob CR", func() {
	const namespace = "default"

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.JetJob{}, &hazelcastv1alpha1.JetJobList{}, namespace, map[string]string{})
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("JetJob create validation", func() {
		It("should not create JetJob with State not Running", Label("fast"), func() {
			jj := &hazelcastv1alpha1.JetJob{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.JetJobSpec{
					Name:                  "jetjobname",
					HazelcastResourceName: "hazelcast",
					State:                 hazelcastv1alpha1.SuspendedJobState,
					JarName:               "myjob.jar",
				},
			}

			Expect(k8sClient.Create(context.Background(), jj)).
				Should(MatchError(ContainSubstring("Invalid value: \"Suspended\": should be set to Running on creation")))
		})

		It("should error when secretName is empty", Label("fast"), func() {
			jj := &hazelcastv1alpha1.JetJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jetjob-1",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.JetJobSpec{
					Name:                  "jetjobname",
					HazelcastResourceName: "hazelcast",
					State:                 hazelcastv1alpha1.SuspendedJobState,
					JarName:               "myjob.jar",
					JetRemoteFileConfiguration: hazelcastv1alpha1.JetRemoteFileConfiguration{
						BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
							BucketURI:  "gs://my-bucket",
							SecretName: "",
						},
					},
				},
			}

			Expect(k8sClient.Create(context.Background(), jj)).
				Should(MatchError(ContainSubstring("bucket secret must be set")))
		})
	})

	Context("JetJob update validation", func() {
		It("should not update immutable fields", Label("fast"), func() {
			spec := hazelcastv1alpha1.JetJobSpec{
				Name:                        "jetjobname",
				HazelcastResourceName:       "hazelcast",
				State:                       hazelcastv1alpha1.RunningJobState,
				JarName:                     "myjob.jar",
				MainClass:                   "com.example.MyClass",
				InitialSnapshotResourceName: "snapshot",
				JetRemoteFileConfiguration: hazelcastv1alpha1.JetRemoteFileConfiguration{
					BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
						SecretName: "my-secret",
						BucketURI:  "gs://my-bucket",
					},
				},
			}
			js, _ := json.Marshal(spec)
			jj := &hazelcastv1alpha1.JetJob{
				ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(js)),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())

			jj.Spec.Name = "newname"
			jj.Spec.HazelcastResourceName = "my-hazelcast"
			jj.Spec.JarName = "another.jar"
			jj.Spec.MainClass = "com.example.AnotherClass"
			jj.Spec.InitialSnapshotResourceName = "another-snapshot"
			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = nil
			err := k8sClient.Update(context.Background(), jj)
			Expect(err).Should(And(
				MatchError(ContainSubstring("spec.name")),
				MatchError(ContainSubstring("spec.hazelcastResourceName")),
				MatchError(ContainSubstring("spec.jarName")),
				MatchError(ContainSubstring("spec.mainClass")),
				MatchError(ContainSubstring("spec.initialSnapshotResourceName")),
				MatchError(ContainSubstring("spec.bucketConfiguration")),
				MatchError(ContainSubstring("Forbidden: field cannot be updated")),
			))
		})

		It("should not allow adding bucket configuration", Label("fast"), func() {
			spec := hazelcastv1alpha1.JetJobSpec{
				Name:                  "jetjobname",
				HazelcastResourceName: "hazelcast",
				State:                 hazelcastv1alpha1.RunningJobState,
				JarName:               "myjob.jar",
				MainClass:             "com.example.MyClass",
			}
			js, _ := json.Marshal(spec)
			jj := &hazelcastv1alpha1.JetJob{
				ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(js)),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())

			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastv1alpha1.BucketConfiguration{
				SecretName: "my-secret",
				BucketURI:  "gs://my-bucket",
			}
			err := k8sClient.Update(context.Background(), jj)
			Expect(err).Should(And(
				MatchError(ContainSubstring("spec.bucketConfiguration")),
				MatchError(ContainSubstring("Forbidden: field cannot be added or removed")),
			))
		})
	})
})
