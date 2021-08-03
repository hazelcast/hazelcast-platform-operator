package controllers

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hazelcast controller", func() {
	const (
		hzKeyName = "hazelcast-test"
		finalizer = "hazelcast.com/finalizer"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("Hazelcast CustomResource with default specs", func() {
		It("Should handle CR and sub resources correctly", func() {

			lookupKey := types.NamespacedName{
				Name:      hzKeyName,
				Namespace: "default",
			}

			toCreate := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: hazelcastv1alpha1.HazelcastSpec{
					ClusterSize:      3,
					Repository:       "hazelcast/hazelcast-enterprise",
					Version:          "5.0-BETA-1",
					LicenseKeySecret: "hazelcast-license-key",
				},
			}

			By("Creating the CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetchedCR := &hazelcastv1alpha1.Hazelcast{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedCR)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(fetchedCR.Spec.ClusterSize).Should(Equal(int32(3)))
			Expect(fetchedCR.Spec.Repository).Should(Equal("hazelcast/hazelcast-enterprise"))
			Expect(fetchedCR.Spec.Version).Should(Equal("5.0-BETA-1"))
			Expect(fetchedCR.Spec.LicenseKeySecret).Should(Equal("hazelcast-license-key"))

			By("Ensures that the status is correct")
			Expect(fetchedCR.Status.Phase).Should(Equal(hazelcastv1alpha1.Pending))

			By("Ensuring the finalizer added successfully")
			Expect(fetchedCR.Finalizers).To(ContainElement(finalizer))

			By("Creating the sub resources successfully")
			expectedOwnerReference := metav1.OwnerReference{
				Kind:               "Hazelcast",
				APIVersion:         "hazelcast.com/v1alpha1",
				UID:                fetchedCR.UID,
				Name:               fetchedCR.Name,
				Controller:         pointer.BoolPtr(true),
				BlockOwnerDeletion: pointer.BoolPtr(true),
			}

			fetchedClusterRole := &rbacv1.ClusterRole{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedClusterRole)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			fetchedServiceAccount := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedServiceAccount)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(fetchedServiceAccount.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedClusterRoleBinding)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			fetchedService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedService)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedSts := &v1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedSts)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(dockerImage(fetchedCR)))

			By("Expecting to delete CR successfully")
			Eventually(func() error {
				fetchedCR = &hazelcastv1alpha1.Hazelcast{}
				_ = k8sClient.Get(context.Background(), lookupKey, fetchedCR)
				return k8sClient.Delete(context.Background(), fetchedCR)
			}, timeout, interval).Should(Succeed())

			By("Expecting to CR delete finish")
			Eventually(func() error {
				return k8sClient.Get(context.Background(), lookupKey, &hazelcastv1alpha1.Hazelcast{})
			}, timeout, interval).ShouldNot(Succeed())

			By("Expecting to ClusterRole removed via finalizer")
			Eventually(func() error {
				return k8sClient.Get(context.Background(), lookupKey, &rbacv1.ClusterRole{})
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
