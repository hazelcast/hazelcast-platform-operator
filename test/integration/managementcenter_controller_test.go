package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
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
			Expect(fetchedSts.Spec.Template.Spec.Volumes).Should(BeNil())

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
	Context("ManagementCenter CustomResource with Persistence", func() {
		When("persistence is enabled with existing Volume Claim", func() {
			It("should add existing Volume Claim to statefulset", func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.ManagementCenterSpec{
						Persistence: hazelcastv1alpha1.PersistenceConfiguration{
							Enabled:                 true,
							ExistingVolumeClaimName: "ClaimName",
						},
					},
				}
				Create(mc)
				fetchedCR := EnsureStatus(mc)
				fetchedSts := &v1.StatefulSet{}
				assertExists(lookupKey(fetchedCR), fetchedSts)
				expectedVolume := corev1.Volume{
					Name: n.MancenterStorageName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "ClaimName",
						},
					},
				}
				Expect(fetchedSts.Spec.Template.Spec.Volumes).To(ConsistOf(expectedVolume))
				Expect(fetchedSts.Spec.VolumeClaimTemplates).Should(BeNil())
				expectedVolumeMount := corev1.VolumeMount{
					Name:      n.MancenterStorageName,
					MountPath: "/data",
				}
				Expect(fetchedSts.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(expectedVolumeMount))
				Delete(mc)
			})
		})
	})
	Context("ManagementCenter Image configuration", func() {
		When("ImagePullSecrets are defined", func() {
			It("should pass the values to StatefulSet spec", func() {
				pullSecrets := []corev1.LocalObjectReference{
					{Name: "mc-secret1"},
					{Name: "mc-secret2"},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.ManagementCenterSpec{
						ImagePullSecrets: pullSecrets,
					},
				}
				Create(mc)
				EnsureStatus(mc)
				fetchedSts := &v1.StatefulSet{}
				assertExists(types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, fetchedSts)
				Expect(fetchedSts.Spec.Template.Spec.ImagePullSecrets).Should(Equal(pullSecrets))
				Delete(mc)
			})
		})
	})

	Context("Pod scheduling parameters", func() {
		When("NodeSelector is used", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.ManagementCenterSpec(defaultSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					NodeSelector: map[string]string{
						"node.selector": "1",
					},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() map[string]string {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.NodeSelector
				}, timeout, interval).Should(HaveKeyWithValue("node.selector", "1"))

				Delete(mc)
			})
		})

		When("Affinity is used", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.ManagementCenterSpec(defaultSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					Affinity: corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{Key: "node.gpu", Operator: corev1.NodeSelectorOpExists},
										},
									},
								},
							},
						},
						PodAffinity: &corev1.PodAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 10,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "node.zone",
									},
								},
							},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 10,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "node.zone",
									},
								},
							},
						},
					},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() *corev1.Affinity {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.Affinity
				}, timeout, interval).Should(Equal(&spec.Scheduling.Affinity))

				Delete(mc)
			})
		})

		When("Toleration is used", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.ManagementCenterSpec(defaultSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.zone",
							Operator: corev1.TolerationOpExists,
						},
					},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() []corev1.Toleration {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.Tolerations
				}, timeout, interval).Should(Equal(spec.Scheduling.Tolerations))

				Delete(mc)
			})
		})
	})
})
