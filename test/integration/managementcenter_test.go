package integration

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("ManagementCenter CR", func() {
	const namespace = "default"

	Create := func(mc *hazelcastv1alpha1.ManagementCenter) {
		By("creating the ManagementCenter CR with specs successfully")
		Expect(k8sClient.Create(context.Background(), mc)).Should(Succeed())
	}

	Update := func(mc *hazelcastv1alpha1.ManagementCenter) {
		By("updating the ManagementCenter CR with specs successfully")
		Expect(k8sClient.Update(context.Background(), mc)).Should(Succeed())
	}

	Fetch := func(mc *hazelcastv1alpha1.ManagementCenter) *hazelcastv1alpha1.ManagementCenter {
		By("fetching Management Center")
		fetchedCR := &hazelcastv1alpha1.ManagementCenter{}
		assertExists(types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, fetchedCR)
		return fetchedCR
	}

	EnsureStatusIsPending := func(mc *hazelcastv1alpha1.ManagementCenter) *hazelcastv1alpha1.ManagementCenter {
		By("ensuring that the status is correct")
		Eventually(func() hazelcastv1alpha1.MCPhase {
			mc = Fetch(mc)
			return mc.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.McPending))
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

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.ManagementCenter{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should create CR with default values when empty specs are applied", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
			}
			Create(mc)
			fetchedCR := EnsureStatusIsPending(mc)
			test.CheckManagementCenterCR(fetchedCR, defaultMcSpecValues(), false)
		})

		It("Should handle CR and sub resources correctly", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
			}

			Create(mc)
			fetchedCR := EnsureStatusIsPending(mc)
			test.CheckManagementCenterCR(fetchedCR, defaultMcSpecValues(), ee)

			Expect(fetchedCR.Spec.HazelcastClusters).Should(BeNil())

			expectedExternalConnectivity := hazelcastv1alpha1.ExternalConnectivityConfiguration{
				Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
			}
			Expect(*fetchedCR.Spec.ExternalConnectivity).Should(Equal(expectedExternalConnectivity))

			expectedPersistence := hazelcastv1alpha1.MCPersistenceConfiguration{
				Enabled: pointer.Bool(true),
				Size:    &[]resource.Quantity{resource.MustParse("10Gi")}[0],
			}
			Expect(*fetchedCR.Spec.Persistence).Should(Equal(expectedPersistence))

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
		})

		When("applying empty spec", func() {
			It("should create CR with default values", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
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
			})
		})
	})

	Context("with ExternalConnectivity configuration", func() {
		It("should create and update service correctly", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
			}

			Create(mc)
			fetchedMc := EnsureStatusIsPending(mc)
			test.CheckManagementCenterCR(fetchedMc, defaultMcSpecValues(), ee)
			EnsureServiceType(mc, corev1.ServiceTypeLoadBalancer)

			fetchedMc.Spec.ExternalConnectivity.Type = hazelcastv1alpha1.ExternalConnectivityTypeNodePort
			Update(fetchedMc)
			fetchedMc = EnsureStatusIsPending(mc)
			EnsureServiceType(mc, corev1.ServiceTypeNodePort)

			fetchedMc.Spec.ExternalConnectivity.Type = hazelcastv1alpha1.ExternalConnectivityTypeClusterIP
			Update(fetchedMc)
			EnsureStatusIsPending(mc)
			EnsureServiceType(mc, corev1.ServiceTypeClusterIP)
		})

		It("should handle Ingress correctly", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
			}

			Create(mc)
			fetchedMc := EnsureStatusIsPending(mc)
			test.CheckManagementCenterCR(fetchedMc, defaultMcSpecValues(), ee)

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
			fetchedMc = EnsureStatusIsPending(mc)

			expectedExternalConnectivityIngress := externalConnectivityIngress.DeepCopy()
			expectedExternalConnectivityIngress.Path = "/"
			Expect(fetchedMc.Spec.ExternalConnectivity.Ingress).Should(Equal(expectedExternalConnectivityIngress))
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
				Path:             "/mc",
			}
			fetchedMc.Spec.ExternalConnectivity.Ingress = updatedExternalConnectivityIngress
			Update(fetchedMc)
			fetchedMc = EnsureStatusIsPending(mc)
			Expect(fetchedMc.Spec.ExternalConnectivity.Ingress).Should(Equal(updatedExternalConnectivityIngress))
			assertExistsAndBeAsExpected(lookupKey(mc), ing, func(ing *networkingv1.Ingress) bool {
				return *ing.Spec.IngressClassName == updatedExternalConnectivityIngress.IngressClassName
			})
			Expect(ing.Annotations).Should(Equal(updatedExternalConnectivityIngress.Annotations))
			Expect(ing.Spec.Rules).Should(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).Should(Equal(updatedExternalConnectivityIngress.Hostname))
			Expect(ing.Spec.Rules[0].HTTP.Paths).Should(HaveLen(1))
			Expect(ing.Spec.Rules[0].HTTP.Paths[0].Path).Should(Equal(updatedExternalConnectivityIngress.Path))
			Expect(*ing.Spec.Rules[0].HTTP.Paths[0].PathType).Should(Equal(networkingv1.PathTypePrefix))
			Expect(ing.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedMc.Spec.ExternalConnectivity.Ingress = nil
			Update(fetchedMc)
			Expect(fetchedMc.Spec.ExternalConnectivity.Ingress).Should(BeNil())
			EnsureStatusIsPending(mc)
			assertDoesNotExist(lookupKey(mc), ing)
		})

		It("should configure contextPath when custom path is set in Ingress", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
			}

			mc.Spec.ExternalConnectivity = &hazelcastv1alpha1.ExternalConnectivityConfiguration{
				Ingress: &hazelcastv1alpha1.ExternalConnectivityIngress{
					IngressClassName: "nginx",
					Annotations:      map[string]string{"app": "hazelcast-mc"},
					Hostname:         "mancenter",
					Path:             "/mc",
				},
			}

			Create(mc)
			fetchedMc := EnsureStatusIsPending(mc)
			test.CheckManagementCenterCR(fetchedMc, defaultMcSpecValues(), ee)

			By("checking contextPath configuration")
			fetchedSts := &appsv1.StatefulSet{}
			assertExists(lookupKey(fetchedMc), fetchedSts)
			Expect(fetchedSts.Spec.Template.Spec.Containers).To(HaveLen(1))
			assertEnvVar(fetchedSts.Spec.Template.Spec.Containers[0], "JAVA_OPTS", func(envvar corev1.EnvVar) bool {
				return strings.Contains(envvar.Value, "-Dhazelcast.mc.contextPath=/mc")
			})

			By("checking liveness probe path")
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].LivenessProbe.HTTPGet.Path).Should(Equal("/mc/health"))
		})

		It("should fail if ingress path is not an absolute path", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
			}

			mc.Spec.ExternalConnectivity = &hazelcastv1alpha1.ExternalConnectivityConfiguration{
				Ingress: &hazelcastv1alpha1.ExternalConnectivityIngress{
					IngressClassName: "nginx",
					Annotations:      map[string]string{"app": "hazelcast-mc"},
					Hostname:         "mancenter",
					Path:             "mc",
				},
			}

			Expect(k8sClient.Create(context.Background(), mc)).
				Should(MatchError(ContainSubstring("must be an absolute path")))
		})
	})

	Context("with Persistence configuration", func() {
		When("persistence is enabled with existing Volume Claim", func() {
			It("should add existing Volume Claim to statefulset", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.ManagementCenterSpec{
						Persistence: &hazelcastv1alpha1.MCPersistenceConfiguration{
							Enabled:                 pointer.Bool(true),
							ExistingVolumeClaimName: "ClaimName",
						},
					},
				}
				Create(mc)
				fetchedCR := EnsureStatusIsPending(mc)
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
			})
		})
	})

	Context("with Image configuration", func() {
		When("ImagePullSecrets are defined", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				pullSecrets := []corev1.LocalObjectReference{
					{Name: "mc-secret1"},
					{Name: "mc-secret2"},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.ManagementCenterSpec{
						ImagePullSecrets: pullSecrets,
					},
				}
				Create(mc)
				EnsureStatusIsPending(mc)
				fetchedSts := &appsv1.StatefulSet{}
				assertExists(types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, fetchedSts)
				Expect(fetchedSts.Spec.Template.Spec.ImagePullSecrets).Should(Equal(pullSecrets))
			})
		})
	})

	Context("with Scheduling configuration", func() {
		When("NodeSelector is given", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.ManagementCenterSpec(defaultMcSpecValues(), ee)
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					NodeSelector: map[string]string{
						"node.selector": "1",
					},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() map[string]string {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.NodeSelector
				}, timeout, interval).Should(HaveKeyWithValue("node.selector", "1"))
			})
		})

		When("Affinity is given", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.ManagementCenterSpec(defaultMcSpecValues(), ee)
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
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
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() *corev1.Affinity {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.Affinity
				}, timeout, interval).Should(Equal(spec.Scheduling.Affinity))
			})
		})

		When("Toleration is given", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.ManagementCenterSpec(defaultMcSpecValues(), ee)
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.zone",
							Operator: corev1.TolerationOpExists,
						},
					},
				}
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Create(mc)

				Eventually(func() []corev1.Toleration {
					ss := getStatefulSet(mc)
					return ss.Spec.Template.Spec.Tolerations
				}, timeout, interval).Should(Equal(spec.Scheduling.Tolerations))
			})
		})
	})

	Context("with Resources parameters", func() {
		When("resources are used", func() {
			It("should be set to Container spec", Label("fast"), func() {
				spec := test.ManagementCenterSpec(defaultMcSpecValues(), ee)
				spec.Resources = &corev1.ResourceRequirements{
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
					ObjectMeta: randomObjectMeta(namespace),
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
			})
		})
	})

	Context("with LDAP security provider", func() {
		When("LDAP security provider is configured", func() {
			It("should be enabled", Label("fast"), func() {
				ldapSecret := CreateLdapSecret("ldap-credential", namespace)
				assertExists(lookupKey(ldapSecret), ldapSecret)
				defer DeleteIfExists(lookupKey(ldapSecret), ldapSecret)
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.SecurityProviders = &hazelcastv1alpha1.SecurityProviders{
					LDAP: &hazelcastv1alpha1.LDAPProvider{
						URL:                   "ldap://10.124.0.27:1389",
						CredentialsSecretName: ldapSecret.Name,
						GroupDN:               "ou=users,dc=example,dc=org",
						GroupSearchFilter:     "member={0}",
						NestedGroupSearch:     false,
						UserDN:                "ou=users,dc=example,dc=org",
						UserGroups:            []string{"readers"},
						MetricsOnlyGroups:     []string{"readers"},
						AdminGroups:           []string{"readers"},
						ReadonlyUserGroups:    []string{"readers"},
						UserSearchFilter:      "cn={0}",
					},
				}
				Create(mc)
				EnsureStatusIsPending(mc)
			})

			It("should error when credentialsSecretName is empty", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.SecurityProviders = &hazelcastv1alpha1.SecurityProviders{
					LDAP: &hazelcastv1alpha1.LDAPProvider{
						URL:                   "ldap://10.124.0.27:1389",
						CredentialsSecretName: "",
						GroupDN:               "ou=users,dc=example,dc=org",
						GroupSearchFilter:     "member={0}",
						NestedGroupSearch:     false,
						UserDN:                "ou=users,dc=example,dc=org",
						UserGroups:            []string{"readers"},
						MetricsOnlyGroups:     []string{"readers"},
						AdminGroups:           []string{"readers"},
						ReadonlyUserGroups:    []string{"readers"},
						UserSearchFilter:      "cn={0}",
					},
				}

				Expect(k8sClient.Create(context.Background(), mc)).
					Should(MatchError(ContainSubstring("Management Center LDAP credentials Secret name is empty")))
			})

			It("should error when credentialsSecretName does not exist", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.SecurityProviders = &hazelcastv1alpha1.SecurityProviders{
					LDAP: &hazelcastv1alpha1.LDAPProvider{
						URL:                   "ldap://10.124.0.27:1389",
						CredentialsSecretName: "ldap-credential",
						GroupDN:               "ou=users,dc=example,dc=org",
						GroupSearchFilter:     "member={0}",
						NestedGroupSearch:     false,
						UserDN:                "ou=users,dc=example,dc=org",
						UserGroups:            []string{"readers"},
						MetricsOnlyGroups:     []string{"readers"},
						AdminGroups:           []string{"readers"},
						ReadonlyUserGroups:    []string{"readers"},
						UserSearchFilter:      "cn={0}",
					},
				}

				Expect(k8sClient.Create(context.Background(), mc)).
					Should(MatchError(ContainSubstring("Management Center LDAP credentials Secret not found")))
			})
		})
	})

	Context("with cluster TLS configuration", func() {
		When("cluster TLS property is configured", func() {
			It("should be enabled", Label("fast"), func() {
				tlsSecret := CreateTLSSecret("tls-secret", namespace)
				assertExists(lookupKey(tlsSecret), tlsSecret)
				defer DeleteIfExists(lookupKey(tlsSecret), tlsSecret)

				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.HazelcastClusters = []hazelcastv1alpha1.HazelcastClusterConfig{{
					Name:    "dev",
					Address: "dummy",
					TLS: &hazelcastv1alpha1.TLS{
						SecretName: tlsSecret.GetName(),
					},
				}}
				Create(mc)
				EnsureStatusIsPending(mc)
			})

			It("should error when secretName is empty", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.HazelcastClusters = []hazelcastv1alpha1.HazelcastClusterConfig{{
					Name:    "dev",
					Address: "dummy",
					TLS: &hazelcastv1alpha1.TLS{
						SecretName: "",
					},
				}}

				Expect(k8sClient.Create(context.Background(), mc)).
					Should(MatchError(ContainSubstring("Management Center Cluster config TLS Secret name is empty")))
			})

			It("should error when secretName does not exist", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.HazelcastClusters = []hazelcastv1alpha1.HazelcastClusterConfig{{
					Name:    "dev",
					Address: "dummy",
					TLS: &hazelcastv1alpha1.TLS{
						SecretName: "notfound",
					},
				}}

				Expect(k8sClient.Create(context.Background(), mc)).
					Should(MatchError(ContainSubstring("Management Center Cluster config TLS Secret not found")))
			})
		})

		When("MutualAuthentication is configured", func() {
			It("should be enabled", Label("fast"), func() {
				tlsSecret := CreateTLSSecret("tls-secret", namespace)
				assertExists(lookupKey(tlsSecret), tlsSecret)
				defer DeleteIfExists(lookupKey(tlsSecret), tlsSecret)

				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
				}
				mc.Spec.HazelcastClusters = []hazelcastv1alpha1.HazelcastClusterConfig{{
					Name:    "dev",
					Address: "dummy",
					TLS: &hazelcastv1alpha1.TLS{
						SecretName:           tlsSecret.GetName(),
						MutualAuthentication: hazelcastv1alpha1.MutualAuthenticationRequired,
					},
				}}
				Create(mc)
				EnsureStatusIsPending(mc)
			})
		})
	})

	Context("StatefulSet", func() {
		firstSpec := hazelcastv1alpha1.ManagementCenterSpec{
			Repository:           "hazelcast/management-center-1",
			Version:              "5.2",
			ImagePullPolicy:      corev1.PullAlways,
			ImagePullSecrets:     nil,
			LicenseKeySecretName: "key-secret",
			HazelcastClusters:    nil,
		}
		secondSpec := hazelcastv1alpha1.ManagementCenterSpec{
			Repository:      "hazelcast/management-center",
			Version:         "5.3",
			ImagePullPolicy: corev1.PullIfNotPresent,
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "secret1"},
				{Name: "secret2"},
			},

			LicenseKeySecretName: "",
			HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
				{Name: "dev", Address: "cluster-address"},
			},

			Scheduling: &hazelcastv1alpha1.SchedulingConfiguration{
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
			Resources: &corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("5Gi"),
				},
			},
		}

		When("updating", func() {
			It("should forward changes to StatefulSet", Label("fast"), func() {
				mc := &hazelcastv1alpha1.ManagementCenter{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       firstSpec,
				}

				Create(mc)
				mc = EnsureStatusIsPending(mc)
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
							Expect(env.Value).To(ContainSubstring(fmt.Sprintf("--client-config /config/%s.yaml", cl.Name)))

						}
					}
				}
				By("checking if StatefulSet LicenseKeySecretName is updated")
				for _, env := range el {
					if env.Name == "MC_LICENSEKEY" {
						Expect(env.ValueFrom.SecretKeyRef.Key).To(Equal(secondSpec.GetLicenseKeySecretName()))
					}
				}

				By("checking if StatefulSet Scheduling is updated")
				Expect(*ss.Spec.Template.Spec.Affinity).To(Equal(*secondSpec.Scheduling.Affinity))
				Expect(ss.Spec.Template.Spec.NodeSelector).To(Equal(secondSpec.Scheduling.NodeSelector))
				Expect(ss.Spec.Template.Spec.Tolerations).To(Equal(secondSpec.Scheduling.Tolerations))
				Expect(ss.Spec.Template.Spec.TopologySpreadConstraints).To(Equal(secondSpec.Scheduling.TopologySpreadConstraints))

				By("checking if StatefulSet Resources is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].Resources).To(Equal(*secondSpec.Resources))
			})
		})
	})

	Context("JVM args configuration", func() {
		It("should set the given jvm args to java opts env in pod template", Label("fast"), func() {
			jvmArgs := []string{
				"-arg1=value1",
				"-arg2=value2",
				"-arg3=value3",
			}

			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					JVM: &hazelcastv1alpha1.MCJVMConfiguration{
						Args: jvmArgs,
					},
				},
			}
			Create(mc)
			mc = EnsureStatusIsPending(mc)

			Expect(k8sClient.Get(context.Background(), lookupKey(mc), mc)).Should(Succeed())
			Expect(mc.Spec.JVM.Args).Should(Equal(jvmArgs))

			sts := appsv1.StatefulSet{}
			Expect(k8sClient.Get(context.Background(), lookupKey(mc), &sts)).Should(Succeed())
			envs := sts.Spec.Template.Spec.Containers[0].Env
			matched := false
			for i := 0; i < len(envs) && !matched; i++ {
				envVars := strings.Split(envs[i].Value, " ")
				argsMatched, err := ContainElements(jvmArgs).Match(envVars)
				if err != nil {
					Fail(err.Error())
				}
				matched = argsMatched
			}
			Expect(matched).To(BeTrue())
		})
	})

	Context("with labels and annotations", func() {
		It("should set labels and annotations to sub-resources", Label("fast"), func() {
			mc := &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.ManagementCenterSpec(defaultMcSpecValues(), ee),
			}
			mc.Spec.Annotations = map[string]string{
				"annotation-example": "hazelcast",
			}
			mc.Spec.Labels = map[string]string{
				//user labels
				"label-example": "hazelcast",

				// reserved labels
				n.ApplicationNameLabel:         "user",
				n.ApplicationInstanceNameLabel: "user",
				n.ApplicationManagedByLabel:    "user",
			}

			Create(mc)
			EnsureStatusIsPending(mc)

			resources := map[string]client.Object{
				"Secret":      &corev1.Secret{},
				"Service":     &corev1.Service{},
				"StatefulSet": &v1.StatefulSet{},
			}
			for name, r := range resources {
				assertExists(lookupKey(mc), r)
				// make sure user annotations and labels are present
				Expect(r.GetAnnotations()).To(HaveKeyWithValue("annotation-example", "hazelcast"), name)
				Expect(r.GetLabels()).To(HaveKeyWithValue("label-example", "hazelcast"), name)

				// make sure operator overwrites selector labels
				Expect(r.GetLabels()).To(Not(HaveKeyWithValue(n.ApplicationNameLabel, "user")), name)
				Expect(r.GetLabels()).To(Not(HaveKeyWithValue(n.ApplicationInstanceNameLabel, "user")), name)
				Expect(r.GetLabels()).To(Not(HaveKeyWithValue(n.ApplicationManagedByLabel, "user")), name)
			}
		})
	})
})
