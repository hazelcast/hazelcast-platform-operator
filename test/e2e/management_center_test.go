package e2e

import (
	"context"
	"fmt"
	"net/http"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
)

var _ = Describe("Management-Center", Label("mc"), func() {
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}

		if runningLocally() {
			return
		}
		By("checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.ManagementCenter{}, nil, hzNamespace, labels)
		deletePVCs(mcLookupKey)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	create := func(mancenter *hazelcastcomv1alpha1.ManagementCenter) {
		By("creating ManagementCenter CR", func() {
			Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
		})

		By("checking ManagementCenter CR running", func() {
			mc := &hazelcastcomv1alpha1.ManagementCenter{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), mcLookupKey, mc)
				Expect(err).ToNot(HaveOccurred())
				return isManagementCenterRunning(mc)
			}, 5*Minute, interval).Should(BeTrue())
		})
	}

	createWithoutCheck := func(mancenter *hazelcastcomv1alpha1.ManagementCenter) {
		By("creating ManagementCenter CR", func() {
			Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
		})
	}

	Describe("Default ManagementCenter CR", func() {
		It("Should create ManagementCenter resources", Label("fast"), func() {
			setLabelAndCRName("mc-1")
			mc := mcconfig.Default(mcLookupKey, ee, labels)
			create(mc)

			By("checking if it created PVC with correct size", func() {
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

			By("asserting status of external addresses are not empty", func() {
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
			setLabelAndCRName("mc-2")
			mc := mcconfig.PersistenceDisabled(mcLookupKey, ee, labels)
			create(mc)

			By("checking if PVC doesn't exist", func() {
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
			setLabelAndCRName("mc-3")
			createWithoutCheck(mcconfig.Faulty(mcLookupKey, ee, labels))
			assertStatusEventually(hazelcastcomv1alpha1.Failed)
		})
	})

	Describe("ManagementCenter CR with Route", func() {
		It("Should be able to access route", Label("fast"), func() {
			if platform.GetType() != platform.OpenShift {
				Skip("This test will only run in OpenShift environments")
			}
			setLabelAndCRName("mc-4")
			mc := mcconfig.RouteEnabled(mcLookupKey, ee, labels)
			create(mc)

			route := &routev1.Route{}
			Expect(k8sClient.Get(context.Background(), mcLookupKey, route)).Should(BeNil())
			resp, err := http.Get("http://" + route.Spec.Host)
			Expect(err).To(BeNil())
			Expect(resp.StatusCode).To(Equal(200))
		})
	})
})
