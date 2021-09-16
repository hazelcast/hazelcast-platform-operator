package e2e

import (
	"context"
	"fmt"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mcName = "managementcenter"
)

var (
	expectedCR = hazelcastcomv1alpha1.ManagementCenter{
		Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
			Repository:       "hazelcast/management-center",
			Version:          "5.0-BETA-2",
			LicenseKeySecret: "hazelcast-license-key",
			ExternalConnectivity: hazelcastcomv1alpha1.ExternalConnectivityConfiguration{
				Type: "LoadBalancer",
			},
			HazelcastClusters: []hazelcastcomv1alpha1.HazelcastClusterConfig{
				{
					Address: "hazelcast",
					Name:    "dev",
				},
			},
			Persistence: hazelcastcomv1alpha1.PersistenceConfiguration{
				Enabled: true,
				Size:    resource.MustParse("10Gi"),
			},
		},
	}
)

var _ = Describe("Management-Center", func() {

	var lookupKey = types.NamespacedName{
		Name:      mcName,
		Namespace: hzNamespace,
	}

	var controllerManagerName = types.NamespacedName{
		Name:      "hazelcast-enterprise-controller-manager",
		Namespace: hzNamespace,
	}

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyManagementCenter(), client.PropagationPolicy(v1.DeletePropagationBackground))).Should(Succeed())
		waitDeletionOf(lookupKey, &hazelcastcomv1alpha1.ManagementCenter{})
		waitDeletionOf(lookupKey, &appsv1.StatefulSet{})
		waitDeletionOf(lookupKey, &corev1.Service{})

		pvcToDelete := &corev1.PersistentVolumeClaim{}
		pvcLookupKey := types.NamespacedName{
			Name:      fmt.Sprintf("mancenter-storage-%s-0", lookupKey.Name),
			Namespace: lookupKey.Namespace,
		}
		deleteIfExists(pvcLookupKey, pvcToDelete)
	})

	Describe("Creating ManagementCenter CR from _v1alpha1_managamentcenter.yaml", func() {
		It("Should create statefulset, service and PVC with correct fields", func() {

			By("Checking hazelcast-enterprise-controller-manager running", func() {
				controllerDep := &appsv1.Deployment{}
				Eventually(func() (int32, error) {
					return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
				}, timeout, interval).Should(Equal(int32(1)))
			})

			mancenter := emptyManagementCenter()
			err := loadFromFile(mancenter, "_v1alpha1_managementcenter.yaml")
			Expect(err).ToNot(HaveOccurred())

			By("Creating Management Center CR", func() {
				Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
			})

			By("Checking ManagementCenter CR has correct fields", func() {
				fetchedCR := &hazelcastcomv1alpha1.ManagementCenter{}
				waitCreationOf(lookupKey, fetchedCR)

				By("Checking Image name", func() {
					Expect(fetchedCR.DockerImage()).Should(Equal(expectedCR.DockerImage()))
				})
				By("Checking License Key", func() {
					Expect(fetchedCR.Spec.LicenseKeySecret).Should(Equal(expectedCR.Spec.LicenseKeySecret))
				})
				By("Checking HazelcastClusters config", func() {
					Expect(fetchedCR.Spec.HazelcastClusters).Should(Equal(expectedCR.Spec.HazelcastClusters))
				})
				By("Checking External connectivity", func() {
					Expect(fetchedCR.Spec.ExternalConnectivity).Should(Equal(expectedCR.Spec.ExternalConnectivity))
				})
				By("Checking Persistence", func() {
					Expect(fetchedCR.Spec.Persistence).Should(Equal(expectedCR.Spec.Persistence))
				})
			})

			By("Checking if StatefulSet created correct resources", func() {
				fetchedSts := &appsv1.StatefulSet{}
				waitCreationOf(lookupKey, fetchedSts)

				By("Checking if it has 1 ready pod", func() {
					waitForStsReady(lookupKey, fetchedSts, int32(1))
				})

				By("Checking if it created PVC with correct size", func() {
					fetchedPVC := &corev1.PersistentVolumeClaim{}
					pvcLookupKey := types.NamespacedName{
						Name:      fmt.Sprintf("mancenter-storage-%s-0", lookupKey.Name),
						Namespace: lookupKey.Namespace,
					}
					waitCreationOf(pvcLookupKey, fetchedPVC)

					expectedResourceList := corev1.ResourceList{
						corev1.ResourceStorage: expectedCR.Spec.Persistence.Size,
					}
					Expect(fetchedPVC.Status.Capacity).Should(Equal(expectedResourceList))
				})
			})

			By("Checking if Service has correct fields", func() {
				fetchedSvc := &corev1.Service{}
				waitCreationOf(lookupKey, fetchedSvc)

				By("Checking if service type is correct", func() {
					Expect(fetchedSvc.Spec.Type).Should(Equal(corev1.ServiceType(expectedCR.Spec.ExternalConnectivity.Type)))
				})
			})
		})
	})
})

func emptyManagementCenter() *hazelcastcomv1alpha1.ManagementCenter {
	return &hazelcastcomv1alpha1.ManagementCenter{
		ObjectMeta: v1.ObjectMeta{
			Name:      mcName,
			Namespace: hzNamespace,
		},
	}
}
