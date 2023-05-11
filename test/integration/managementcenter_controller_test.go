package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
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

	Create := func(obj client.Object) {
		By("creating the CR with specs successfully")
		Expect(k8sClient.Create(context.Background(), obj)).Should(Succeed())
	}

	Update := func(mc *hazelcastv1alpha1.ManagementCenter) {
		By("updating the CR with specs successfully")
		Expect(k8sClient.Update(context.Background(), mc)).Should(Succeed())
	}

	Delete := func(obj client.Object) {
		By("expecting to delete CR successfully")
		deleteIfExists(lookupKey(obj), obj)

		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey(obj), obj)
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

	EnsureServiceType := func(mc *hazelcastv1alpha1.ManagementCenter, svcType corev1.ServiceType) *corev1.Service {
		By("ensuring that the status is correct")
		svc := &corev1.Service{}
		Eventually(func() corev1.ServiceType {
			assertExists(lookupKey(mc), svc)
			return svc.Spec.Type
		}, timeout, interval).Should(Equal(svcType))
		return svc
	}

	CreateLicenseKeySecret := func(name string) *corev1.Secret {
		By(fmt.Sprintf("creating license key secret '%s'", name))
		licenseSec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				n.LicenseDataKey: []byte("integration-test-license"),
			},
		}

		Eventually(func() bool {
			err := k8sClient.Create(context.Background(), licenseSec)
			return err == nil || errors.IsAlreadyExists(err)
		}, timeout, interval).Should(BeTrue())

		assertExists(lookupKey(licenseSec), &corev1.Secret{})

		return licenseSec
	}

	BeforeEach(func() {
		if ee {
			CreateLicenseKeySecret(n.LicenseKeySecret)
		}
	})

	Context("ManagementCenter CustomResource with default specs", func() {
		It("should create CR with default values when empty specs are applied", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: GetRandomObjectMeta(),
			}
			Create(mc)
			fetchedCR := EnsureStatus(mc)
			test.CheckManagementCenterCR(fetchedCR, defaultSpecValues, false)
			Delete(mc)
		})

		It("Should handle CR and sub resources correctly", Label("fast"), func() {
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

			expectedPersistence := hazelcastv1alpha1.MCPersistenceConfiguration{
				Enabled: pointer.Bool(true),
				Size:    &[]resource.Quantity{resource.MustParse("10Gi")}[0],
			}
			Expect(fetchedCR.Spec.Persistence).Should(Equal(expectedPersistence))

			By("creating the sub resources successfully")
			expectedOwnerReference := metav1.OwnerReference{
				Kind:               "ManagementCenter",
				APIVersion:         "hazelcast.com/v1alpha1",
				UID:                fetchedCR.UID,
				Name:               fetchedCR.Name,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			}

			fetchedService := EnsureServiceType(fetchedCR, corev1.ServiceTypeLoadBalancer)
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedSts := &appsv1.StatefulSet{}
			assertExists(lookupKey(fetchedCR), fetchedSts)
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(*fetchedSts.Spec.Replicas).Should(Equal(int32(1)))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(fetchedCR.DockerImage()))
			expectedPVCSpec := corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: nil,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			}
			Expect(fetchedSts.Spec.VolumeClaimTemplates[0].Spec.AccessModes).To(Equal(expectedPVCSpec.AccessModes))
			Expect(fetchedSts.Spec.VolumeClaimTemplates[0].Spec.Resources).To(Equal(expectedPVCSpec.Resources))

			Delete(mc)

		})
		It("should create CR with default values when empty specs are applied", Label("fast"), func() {
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

	Context("ManagementCenter CustomResource with ExternalConnectivity", func() {

		It("should create and update service correctly", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       test.ManagementCenterSpec(defaultSpecValues, ee),
			}

			Create(mc)
			fetchedMc := EnsureStatus(mc)
			test.CheckManagementCenterCR(fetchedMc, defaultSpecValues, ee)
			EnsureServiceType(mc, corev1.ServiceTypeLoadBalancer)

			fetchedMc.Spec.ExternalConnectivity.Type = hazelcastv1alpha1.ExternalConnectivityTypeNodePort
			Update(fetchedMc)
			fetchedMc = EnsureStatus(mc)
			EnsureServiceType(mc, corev1.ServiceTypeNodePort)

			fetchedMc.Spec.ExternalConnectivity.Type = hazelcastv1alpha1.ExternalConnectivityTypeClusterIP
			Update(fetchedMc)
			EnsureStatus(mc)
			EnsureServiceType(mc, corev1.ServiceTypeClusterIP)
		})

		It("should handle ingress correctly", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       test.ManagementCenterSpec(defaultSpecValues, ee),
			}

			Create(mc)
			fetchedMc := EnsureStatus(mc)
			test.CheckManagementCenterCR(fetchedMc, defaultSpecValues, ee)

			ing := &networkingv1.Ingress{}
			assertDoesNotExist(lookupKey(mc), ing)

			externalConnectivityIngress := &hazelcastv1alpha1.ExternalConnectivityIngress{
				IngressClassName: "nginx",
				Annotations:      map[string]string{"app": "hazelcast-mc"},
				Hostname:         "mancenter",
			}
			fetchedMc.Spec.ExternalConnectivity.Ingress = externalConnectivityIngress

			expectedOwnerReference := metav1.OwnerReference{
				Kind:               "ManagementCenter",
				APIVersion:         "hazelcast.com/v1alpha1",
				UID:                fetchedMc.UID,
				Name:               fetchedMc.Name,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			}

			Update(fetchedMc)
			fetchedMc = EnsureStatus(mc)
			Expect(fetchedMc.Spec.ExternalConnectivity.Ingress).Should(Equal(externalConnectivityIngress))
			assertExists(lookupKey(mc), ing)
			Expect(*ing.Spec.IngressClassName).Should(Equal(externalConnectivityIngress.IngressClassName))
			Expect(ing.Annotations).Should(Equal(externalConnectivityIngress.Annotations))
			Expect(ing.Spec.Rules).Should(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).Should(Equal(externalConnectivityIngress.Hostname))
			Expect(ing.Spec.Rules[0].HTTP.Paths).Should(HaveLen(1))
			Expect(ing.Spec.Rules[0].HTTP.Paths[0].Path).Should(Equal("/"))
			Expect(*ing.Spec.Rules[0].HTTP.Paths[0].PathType).Should(Equal(networkingv1.PathTypePrefix))
			Expect(ing.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			updatedExternalConnectivityIngress := &hazelcastv1alpha1.ExternalConnectivityIngress{
				IngressClassName: "traefik",
				Annotations:      map[string]string{"app": "hazelcast-mc", "management-center": "ingress"},
				Hostname:         "mc.app",
			}
			fetchedMc.Spec.ExternalConnectivity.Ingress = updatedExternalConnectivityIngress
			Update(fetchedMc)
			fetchedMc = EnsureStatus(mc)
			Expect(fetchedMc.Spec.ExternalConnectivity.Ingress).Should(Equal(updatedExternalConnectivityIngress))
			assertExistsAndBeAsExpected(lookupKey(mc), ing, func(ing *networkingv1.Ingress) bool {
				return *ing.Spec.IngressClassName == updatedExternalConnectivityIngress.IngressClassName
			})
			Expect(ing.Annotations).Should(Equal(updatedExternalConnectivityIngress.Annotations))
			Expect(ing.Spec.Rules).Should(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).Should(Equal(updatedExternalConnectivityIngress.Hostname))
			Expect(ing.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedMc.Spec.ExternalConnectivity.Ingress = nil
			Update(fetchedMc)
			Expect(fetchedMc.Spec.ExternalConnectivity.Ingress).Should(BeNil())
			EnsureStatus(mc)
			assertDoesNotExist(lookupKey(mc), ing)
		})
	})

	Context("ManagementCenter CustomResource with Persistence", func() {
		When("persistence is enabled with existing Volume Claim", func() {
			It("should add existing Volume Claim to statefulset", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.ManagementCenterSpec{
						Persistence: hazelcastv1alpha1.MCPersistenceConfiguration{
							Enabled:                 pointer.Bool(true),
							ExistingVolumeClaimName: "ClaimName",
						},
					},
				}
				Create(mc)
				fetchedCR := EnsureStatus(mc)
				fetchedSts := &appsv1.StatefulSet{}
				assertExists(lookupKey(fetchedCR), fetchedSts)
				expectedVolume := corev1.Volume{
					Name: n.MancenterStorageName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "ClaimName",
						},
					},
				}
				Expect(fetchedSts.Spec.Template.Spec.Volumes).To(ContainElement(expectedVolume))
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
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
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
				fetchedSts := &appsv1.StatefulSet{}
				assertExists(types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, fetchedSts)
				Expect(fetchedSts.Spec.Template.Spec.ImagePullSecrets).Should(Equal(pullSecrets))
				Delete(mc)
			})
		})
	})

	Context("Pod scheduling parameters", func() {
		When("NodeSelector is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
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
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.ManagementCenterSpec(defaultSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					Affinity: &corev1.Affinity{
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
				}, timeout, interval).Should(Equal(spec.Scheduling.Affinity))

				Delete(mc)
			})
		})

		When("Toleration is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
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

	Context("Resources context", func() {
		When("Resources are used", func() {
			It("should be set to Container spec", Label("fast"), func() {
				spec := test.ManagementCenterSpec(defaultSpecValues, ee)
				spec.Resources = corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("5Gi"),
					},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() map[corev1.ResourceName]resource.Quantity {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.Containers[0].Resources.Limits
				}, timeout, interval).Should(And(
					HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")),
					HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("10Gi"))),
				)

				Eventually(func() map[corev1.ResourceName]resource.Quantity {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.Containers[0].Resources.Requests
				}, timeout, interval).Should(And(
					HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")),
					HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("5Gi"))),
				)

				Delete(mc)
			})
		})
	})

	Context("ManagementCenter cluster TLS configuration", func() {
		When("TLS property is configured", func() {
			It("should be enabled", Label("fast"), func() {
				secret := &corev1.Secret{
					ObjectMeta: GetRandomObjectMeta(),
					Data: map[string][]byte{
						"tls.crt": []byte(exampleCert),
						"tls.key": []byte(exampleKey),
					},
				}
				Create(secret)
				defer Delete(secret)

				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       test.ManagementCenterSpec(defaultSpecValues, ee),
				}
				mc.Spec.HazelcastClusters = []hazelcastv1alpha1.HazelcastClusterConfig{{
					Name:    "dev",
					Address: "dummy",
					TLS: hazelcastv1alpha1.TLS{
						SecretName: secret.GetName(),
					},
				}}
				Create(mc)
				EnsureStatus(mc)
				Delete(mc)
			})
		})
	})

	Context("Statefulset Updates", func() {
		firstSpec := hazelcastv1alpha1.ManagementCenterSpec{
			Repository:        "hazelcast/management-center-1",
			Version:           "5.2",
			ImagePullPolicy:   corev1.PullAlways,
			ImagePullSecrets:  nil,
			LicenseKeySecret:  "key-secret",
			HazelcastClusters: nil,
		}
		secondSpec := hazelcastv1alpha1.ManagementCenterSpec{
			Repository:      "hazelcast/management-center",
			Version:         "5.3",
			ImagePullPolicy: corev1.PullIfNotPresent,
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "secret1"},
				{Name: "secret2"},
			},

			LicenseKeySecret: "",
			HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
				{Name: "dev", Address: "cluster-address"},
			},

			Scheduling: hazelcastv1alpha1.SchedulingConfiguration{
				Affinity: &corev1.Affinity{
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
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("5Gi"),
				},
			},
		}
		When("Management Center Spec is updated", func() {
			It("Should forward changes to StatefulSet", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       firstSpec,
				}

				Create(mc)
				mc = EnsureStatus(mc)
				mc.Spec = secondSpec

				Expect(k8sClient.Update(context.Background(), mc)).Should(Succeed())
				ss := getStatefulSet(mc)

				By("checking if StatefulSet Image is updated")
				Eventually(func() string {
					ss = getStatefulSet(mc)
					return ss.Spec.Template.Spec.Containers[0].Image
				}, timeout, interval).Should(Equal(fmt.Sprintf("%s:%s", secondSpec.Repository, secondSpec.Version)))

				By("checking if StatefulSet ImagePullPolicy is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(secondSpec.ImagePullPolicy))

				By("checking if StatefulSet ImagePullSecrets is updated")
				Expect(ss.Spec.Template.Spec.ImagePullSecrets).To(Equal(secondSpec.ImagePullSecrets))

				By("checking if StatefulSet HazelcastClusters is updated")
				hzcl := mc.Spec.HazelcastClusters
				el := ss.Spec.Template.Spec.Containers[0].Env
				for _, env := range el {
					if env.Name == "MC_INIT_CMD" {
						for _, cl := range hzcl {
							Expect(env.Value).To(ContainSubstring(fmt.Sprintf("--client-config /config/%s.xml", cl.Name)))

						}
					}
				}
				By("checking if StatefulSet LicenseKeySecret is updated")
				for _, env := range el {
					if env.Name == "MC_LICENSEKEY" {
						Expect(env.ValueFrom.SecretKeyRef.Key).To(Equal(secondSpec.LicenseKeySecret))
					}
				}

				By("checking if StatefulSet Scheduling is updated")
				Expect(*ss.Spec.Template.Spec.Affinity).To(Equal(*secondSpec.Scheduling.Affinity))
				Expect(ss.Spec.Template.Spec.NodeSelector).To(Equal(secondSpec.Scheduling.NodeSelector))
				Expect(ss.Spec.Template.Spec.Tolerations).To(Equal(secondSpec.Scheduling.Tolerations))
				Expect(ss.Spec.Template.Spec.TopologySpreadConstraints).To(Equal(secondSpec.Scheduling.TopologySpreadConstraints))

				By("checking if StatefulSet Resources is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].Resources).To(Equal(secondSpec.Resources))

				Delete(mc)
			})
		})
	})
})
