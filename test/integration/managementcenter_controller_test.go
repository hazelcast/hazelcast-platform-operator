package integration

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ManagementCenter controller", func() {
	const (
		mcKeyName = "management-center-test"
	)

	defaultSpecValues := &test.MCSpecValues{
		Repository: n.MCRepo,
		Version:    n.MCVersion,
		LicenseKey: n.LicenseKeySecret,
	}

	lookupKey := types.NamespacedName{
		Name:      mcKeyName,
		Namespace: "default",
	}

	Delete := func() {
		By("expecting to delete CR successfully")
		fetchedCR := &hazelcastv1alpha1.ManagementCenter{}

		deleteIfExists(lookupKey, fetchedCR)

		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey, fetchedCR)
	}

	Context("ManagementCenter CustomResource with default specs", func() {

		It("Should handle CR and sub resources correctly", func() {
			toCreate := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: test.ManagementCenterSpec(defaultSpecValues, ee),
			}

			By("Creating the CR with specs successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetchedCR := &hazelcastv1alpha1.ManagementCenter{}
			assertExists(lookupKey, fetchedCR)

			test.CheckManagementCenterCR(fetchedCR, defaultSpecValues, ee)

			Expect(fetchedCR.Spec.HazelcastClusters).Should(Equal([]hazelcastv1alpha1.HazelcastClusterConfig{}))

			expectedExternalConnectivity := hazelcastv1alpha1.ExternalConnectivityConfiguration{
				Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
			}
			Expect(fetchedCR.Spec.ExternalConnectivity).Should(Equal(expectedExternalConnectivity))

			expectedPersistence := hazelcastv1alpha1.PersistenceConfiguration{
				Enabled:      false,
				StorageClass: nil,
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
			assertExists(lookupKey, fetchedService)
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(fetchedService.Spec.Type).Should(Equal(corev1.ServiceType("LoadBalancer")))

			fetchedSts := &v1.StatefulSet{}
			assertExists(lookupKey, fetchedSts)
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(*fetchedSts.Spec.Replicas).Should(Equal(int32(1)))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(fetchedCR.DockerImage()))
			Expect(fetchedSts.Spec.VolumeClaimTemplates).Should(BeNil())

			Delete()

		})
		It("should create CR with default values when empty specs are applied", func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
				},
			}
			Expect(k8sClient.Create(context.Background(), mc)).Should(Succeed())

			fetchedCR := &hazelcastv1alpha1.ManagementCenter{}
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), lookupKey, fetchedCR)
				if err != nil {
					return ""
				}
				return fetchedCR.Spec.Repository
			}, timeout, interval).Should(Equal(n.MCRepo))
			Expect(fetchedCR.Spec.Version).Should(Equal(n.MCVersion))

			Delete()
		})
	})
})
