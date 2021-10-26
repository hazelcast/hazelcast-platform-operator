package integration

import (
	"context"
	"time"

	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ManagementCenter controller", func() {
	const (
		mcKeyName = "management-center-test"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("ManagementCenter CustomResource with default specs", func() {
		It("Should handle CR and sub resources correctly", func() {

			lookupKey := types.NamespacedName{
				Name:      mcKeyName,
				Namespace: "default",
			}

			toCreate := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					Repository:       n.MCRepo,
					Version:          n.MCVersion,
					LicenseKeySecret: n.LicenseKeySecret,
					ExternalConnectivity: hazelcastv1alpha1.ExternalConnectivityConfiguration{
						Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
					},
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
					Persistence: hazelcastv1alpha1.PersistenceConfiguration{
						StorageClass: &[]string{""}[0],
					},
				},
			}

			By("Creating the CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetchedCR := &hazelcastv1alpha1.ManagementCenter{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedCR)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(fetchedCR.Spec.Repository).Should(Equal(n.MCRepo))
			Expect(fetchedCR.Spec.Version).Should(Equal(n.MCVersion))
			Expect(fetchedCR.Spec.LicenseKeySecret).Should(Equal(n.LicenseKeySecret))

			Expect(fetchedCR.Spec.HazelcastClusters).Should(Equal([]hazelcastv1alpha1.HazelcastClusterConfig{}))

			expectedExternalConnectivity := hazelcastv1alpha1.ExternalConnectivityConfiguration{
				Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
			}
			Expect(fetchedCR.Spec.ExternalConnectivity).Should(Equal(expectedExternalConnectivity))

			expectedPersistence := hazelcastv1alpha1.PersistenceConfiguration{
				Enabled:      false,
				StorageClass: &[]string{""}[0],
				Size:         resource.MustParse("0"),
			}
			Expect(fetchedCR.Spec.Persistence).Should(Equal(expectedPersistence))

			By("Creating the sub resources successfully")
			expectedOwnerReference := metav1.OwnerReference{
				Kind:               "ManagementCenter",
				APIVersion:         "hazelcast.com/v1alpha1",
				UID:                fetchedCR.UID,
				Name:               fetchedCR.Name,
				Controller:         pointer.BoolPtr(true),
				BlockOwnerDeletion: pointer.BoolPtr(true),
			}

			fetchedService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedService)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(fetchedService.Spec.Type).Should(Equal(corev1.ServiceType("LoadBalancer")))

			fetchedSts := &v1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedSts)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(*fetchedSts.Spec.Replicas).Should(Equal(int32(1)))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(fetchedCR.DockerImage()))
			Expect(fetchedSts.Spec.VolumeClaimTemplates).Should(BeNil())

			By("Expecting to delete CR successfully")
			Eventually(func() error {
				fetchedCR = &hazelcastv1alpha1.ManagementCenter{}
				_ = k8sClient.Get(context.Background(), lookupKey, fetchedCR)
				return k8sClient.Delete(context.Background(), fetchedCR)
			}, timeout, interval).Should(Succeed())

			By("Expecting to CR delete finish")
			Eventually(func() error {
				return k8sClient.Get(context.Background(), lookupKey, &hazelcastv1alpha1.ManagementCenter{})
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})
