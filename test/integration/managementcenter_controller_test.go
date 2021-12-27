package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"

	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("ManagementCenter controller", func() {
	const (
		namespace = "default"
	)

	defaultSpecValues := &test.MCSpecValues{
		Repository:      n.MCRepo,
		Version:         n.MCVersion,
		LicenseKey:      n.LicenseKeySecret,
		ImagePullPolicy: n.MCImagePullPolicy,
	}

	lookupKey := func(mc *hazelcastv1alpha1.ManagementCenter) types.NamespacedName {
		return types.NamespacedName{
			Name:      mc.Name,
			Namespace: mc.Namespace,
		}
	}
	GetRandomObjectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("mc-test-%s", uuid.NewUUID()),
			Namespace: namespace,
		}
	}

	Create := func(mc *hazelcastv1alpha1.ManagementCenter) {
		By("creating the CR with specs successfully")
		Expect(k8sClient.Create(context.Background(), mc)).Should(Succeed())
	}

	Delete := func(mc *hazelcastv1alpha1.ManagementCenter) {
		By("expecting to delete CR successfully")
		fetchedCR := &hazelcastv1alpha1.ManagementCenter{}

		deleteIfExists(lookupKey(mc), fetchedCR)

		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey(mc), fetchedCR)
	}

	Fetch := func(mc *hazelcastv1alpha1.ManagementCenter) *hazelcastv1alpha1.ManagementCenter {
		By("fetching Management Center")
		fetchedCR := &hazelcastv1alpha1.ManagementCenter{}
		assertExists(types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, fetchedCR)
		return fetchedCR
	}

	EnsureStatus := func(mc *hazelcastv1alpha1.ManagementCenter) *hazelcastv1alpha1.ManagementCenter {
		By("ensuring that the status is correct")
		Eventually(func() hazelcastv1alpha1.Phase {
			mc = Fetch(mc)
			return mc.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.Pending))
		return mc
	}

	Context("ManagementCenter CustomResource with default specs", func() {
		It("should create CR with default values when empty specs are applied", func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: GetRandomObjectMeta(),
			}
			Create(mc)
			fetchedCR := EnsureStatus(mc)
			test.CheckManagementCenterCR(fetchedCR, defaultSpecValues, false)
			Delete(mc)
		})

		It("Should handle CR and sub resources correctly", func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       test.ManagementCenterSpec(defaultSpecValues, ee),
			}

			Create(mc)
			fetchedCR := EnsureStatus(mc)
			test.CheckManagementCenterCR(fetchedCR, defaultSpecValues, ee)

			Expect(fetchedCR.Spec.HazelcastClusters).Should(BeNil())

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
			assertExists(lookupKey(fetchedCR), fetchedService)
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(fetchedService.Spec.Type).Should(Equal(corev1.ServiceType("LoadBalancer")))

			fetchedSts := &v1.StatefulSet{}
			assertExists(lookupKey(fetchedCR), fetchedSts)
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(*fetchedSts.Spec.Replicas).Should(Equal(int32(1)))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(fetchedCR.DockerImage()))
			Expect(fetchedSts.Spec.VolumeClaimTemplates).Should(BeNil())

			Delete(mc)

		})
		It("should create CR with default values when empty specs are applied", func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: GetRandomObjectMeta(),
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
				},
			}
			Create(mc)

			fetchedCR := &hazelcastv1alpha1.ManagementCenter{}
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), lookupKey(mc), fetchedCR)
				if err != nil {
					return ""
				}
				return fetchedCR.Spec.Repository
			}, timeout, interval).Should(Equal(n.MCRepo))
			Expect(fetchedCR.Spec.Version).Should(Equal(n.MCVersion))

			Delete(mc)
		})
	})
})
