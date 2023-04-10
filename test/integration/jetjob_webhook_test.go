package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

var _ = Describe("Hazelcast webhook", func() {

	Context("JetJob create validation", func() {
		It("should not create JetJob with State not Running", Label("fast"), func() {
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
				},
			}

			Expect(k8sClient.Create(context.Background(), jj)).
				Should(MatchError(ContainSubstring("Invalid value: \"Suspended\": should be set to Running on creation")))
		})
	})

	Context("JetJob update validation", func() {
		It("should not update immutable fields", Label("fast"), func() {
			jj := &hazelcastv1alpha1.JetJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jetjob-2",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.JetJobSpec{
					Name:                  "jetjobname",
					HazelcastResourceName: "hazelcast",
					State:                 hazelcastv1alpha1.RunningJobState,
					JarName:               "myjob.jar",
					MainClass:             "com.example.MyClass",
				},
			}

			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())

			jj.Spec.Name = "newname"
			jj.Spec.HazelcastResourceName = "my-hazelcast"
			jj.Spec.JarName = "another.jar"
			jj.Spec.MainClass = "com.example.AnotherClass"
			err := k8sClient.Update(context.Background(), jj)
			Expect(err).Should(And(
				MatchError(ContainSubstring("spec.name")),
				MatchError(ContainSubstring("spec.hazelcastResourceName")),
				MatchError(ContainSubstring("spec.jarName")),
				MatchError(ContainSubstring("spec.mainClass")),
				MatchError(ContainSubstring("Forbidden: field cannot be updated")),
			))
		})

		It("should not change state Job ID is not assigned", Label("fast"), func() {
			jj := &hazelcastv1alpha1.JetJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "jetjob-3",
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.JetJobSpec{
					Name:                  "jetjobname",
					HazelcastResourceName: "hazelcast",
					State:                 hazelcastv1alpha1.RunningJobState,
					JarName:               "myjob.jar",
					MainClass:             "com.example.MyClass",
				},
				Status: hazelcastv1alpha1.JetJobStatus{
					Id:    123456,
					Phase: hazelcastv1alpha1.JetJobRunning,
				},
			}
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())

			jj2 := &hazelcastv1alpha1.JetJob{}
			k8sClient.Get(context.Background(), types.NamespacedName{Name: "jetjob-3", Namespace: "default"}, jj2)
			fmt.Printf("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ %v", jj2.Status.Id)
		})
	})
})
