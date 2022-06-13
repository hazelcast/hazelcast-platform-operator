package e2e

import (
	"context"
	"fmt"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
)

var _ = Describe("Management-Center", Label("mc"), func() {
	mcName := fmt.Sprintf("managementcenter-%d", GinkgoParallelProcess())

	var mcLookupKey = types.NamespacedName{
		Name:      mcName,
		Namespace: hzNamespace,
	}
	labels := map[string]string{
		"test_suite": fmt.Sprintf("mc-%d", GinkgoParallelProcess()),
	}
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}

		if runningLocally() {
			return
		}
		By("Checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), emptyManagementCenter(mcLookupKey), client.PropagationPolicy(v1.DeletePropagationForeground))).Should(Succeed())
		assertDoesNotExist(mcLookupKey, &hazelcastcomv1alpha1.ManagementCenter{})
		deletePVCs(mcLookupKey)
	})

	create := func(mancenter *hazelcastcomv1alpha1.ManagementCenter) {
		By("Creating ManagementCenter CR", func() {
			Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
		})

		By("Checking ManagementCenter CR running", func() {
			mc := &hazelcastcomv1alpha1.ManagementCenter{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), mcLookupKey, mc)
				Expect(err).ToNot(HaveOccurred())
				return isManagementCenterRunning(mc)
			}, 5*Minute, interval).Should(BeTrue())
		})
	}

	createWithoutCheck := func(mancenter *hazelcastcomv1alpha1.ManagementCenter) {
		By("Creating ManagementCenter CR", func() {
			Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
		})
	}

	Describe("Default ManagementCenter CR", func() {
		It("Should create ManagementCenter resources", Label("fast"), func() {
			mc := mcconfig.Default(mcLookupKey, ee, labels)
			create(mc)

			By("Checking if it created PVC with correct size", func() {
				fetchedPVC := &corev1.PersistentVolumeClaim{}
				pvcLookupKey := types.NamespacedName{
					Name:      fmt.Sprintf("mancenter-storage-%s-0", mcLookupKey.Name),
					Namespace: mcLookupKey.Namespace,
				}
				assertExists(pvcLookupKey, fetchedPVC)

				expectedResourceList := corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				}
				Expect(fetchedPVC.Status.Capacity).Should(Equal(expectedResourceList))
			})

			By("Assert status external addresses not empty", func() {
				Eventually(func() string {
					cr := hazelcastcomv1alpha1.ManagementCenter{}
					err := k8sClient.Get(context.Background(), mcLookupKey, &cr)
					Expect(err).ToNot(HaveOccurred())
					return cr.Status.ExternalAddresses
				}, 5*Minute, interval).Should(Not(BeEmpty()))
			})
		})
	})

	Describe("ManagementCenter CR without Persistence", func() {
		It("Should create ManagementCenter resources and no PVC", Label("fast"), func() {
			mc := mcconfig.PersistenceDisabled(mcLookupKey, ee, labels)
			create(mc)

			By("Checking if PVC does not exist", func() {
				fetchedPVC := &corev1.PersistentVolumeClaim{}
				pvcLookupKey := types.NamespacedName{
					Name:      fmt.Sprintf("mancenter-storage-%s-0", mcLookupKey.Name),
					Namespace: mcLookupKey.Namespace,
				}
				assertDoesNotExist(pvcLookupKey, fetchedPVC)
			})
		})
	})

	Describe("External API errors", func() {
		assertStatusEventually := func(phase hazelcastcomv1alpha1.Phase) {
			mc := &hazelcastcomv1alpha1.ManagementCenter{}
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), mcLookupKey, mc)
				Expect(err).ToNot(HaveOccurred())
				return mc.Status.Phase
			}, 2*Minute, interval).Should(Equal(phase))
			Expect(mc.Status.Message).Should(Not(BeEmpty()))
		}

		It("should be reflected to Management CR status", Label("fast"), func() {
			createWithoutCheck(mcconfig.Faulty(mcLookupKey, ee, labels))
			assertStatusEventually(hazelcastcomv1alpha1.Failed)
		})
	})
})
