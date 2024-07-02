package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/aws/smithy-go/ptr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/controller/hazelcast"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Hazelcast CR", func() {
	const namespace = "default"

	labelFilter := func(hz *hazelcastv1alpha1.Hazelcast) client.MatchingLabels {
		return map[string]string{
			n.ApplicationNameLabel:         n.Hazelcast,
			n.ApplicationManagedByLabel:    n.OperatorName,
			n.ApplicationInstanceNameLabel: hz.Name,
		}
	}

	clusterScopedLookupKey := func(hz *hazelcastv1alpha1.Hazelcast) types.NamespacedName {
		return types.NamespacedName{
			Name:      (&hazelcastv1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{Name: hz.Name, Namespace: namespace}}).ClusterScopedName(),
			Namespace: "",
		}
	}

	create := func(hz *hazelcastv1alpha1.Hazelcast) {
		By("creating the Hazelcast CR with specs successfully")
		Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
	}

	fetchServices := func(hz *hazelcastv1alpha1.Hazelcast, waitForN int) *corev1.ServiceList {
		serviceList := &corev1.ServiceList{}
		Eventually(func() bool {
			err := k8sClient.List(context.Background(), serviceList, client.InNamespace(hz.Namespace), labelFilter(hz))
			if err != nil || len(serviceList.Items) != waitForN {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())
		return serviceList
	}

	type UpdateFn func(*hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast

	setClusterSize := func(size int32) UpdateFn {
		return func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
			hz.Spec.ClusterSize = &size
			return hz
		}
	}

	enableUnisocket := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.Type = hazelcastv1alpha1.ExposeExternallyTypeUnisocket
		hz.Spec.ExposeExternally.MemberAccess = ""
		return hz
	}

	disableMemberAccess := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.MemberAccess = ""
		return hz
	}

	enableSmart := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.Type = hazelcastv1alpha1.ExposeExternallyTypeSmart
		return hz
	}

	setDiscoveryViaLoadBalancer := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.DiscoveryServiceType = corev1.ServiceTypeLoadBalancer
		return hz
	}

	disableExposeExternally := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally = nil
		return hz
	}

	removeSpec := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec = hazelcastv1alpha1.HazelcastSpec{
			LicenseKeySecretName: n.LicenseKeySecret,
		}
		return hz
	}

	update := func(hz *hazelcastv1alpha1.Hazelcast, fns ...UpdateFn) {
		By("updating the CR with specs successfully")
		if len(fns) == 0 {
			Expect(k8sClient.Update(context.Background(), hz)).Should(Succeed())
		} else {
			for {
				cr := &hazelcastv1alpha1.Hazelcast{}
				Expect(k8sClient.Get(
					context.Background(),
					types.NamespacedName{Name: hz.Name, Namespace: hz.Namespace},
					cr),
				).Should(Succeed())
				for _, fn := range fns {
					cr = fn(cr)
				}
				err := k8sClient.Update(context.Background(), cr)
				if err == nil {
					break
				} else if errors.IsConflict(err) {
					continue
				} else {
					Fail(err.Error())
				}
			}
		}
	}

	ensureSpecEquals := func(hz *hazelcastv1alpha1.Hazelcast, other *test.HazelcastSpecValues) *hazelcastv1alpha1.Hazelcast {
		By("ensuring spec is defaulted")
		Eventually(func() *hazelcastv1alpha1.HazelcastSpec {
			hz = fetchHz(hz)
			return &hz.Spec
		}, timeout, interval).Should(test.EqualSpecs(other))
		return hz
	}

	BeforeEach(func() {
		By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
		licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
		assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with default configuration", func() {
		It("should handle CR and sub resources correctly", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

			By("ensuring the finalizer added successfully")
			Expect(fetchedCR.Finalizers).To(ContainElement(n.Finalizer))

			By("creating the sub resources successfully")
			expectedOwnerReference := metav1.OwnerReference{
				Kind:               "Hazelcast",
				APIVersion:         "hazelcast.com/v1alpha1",
				UID:                fetchedCR.UID,
				Name:               fetchedCR.Name,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: pointer.Bool(true),
			}

			fetchedClusterRole := &rbacv1.ClusterRole{}
			assertExists(clusterScopedLookupKey(hz), fetchedClusterRole)

			fetchedServiceAccount := &corev1.ServiceAccount{}
			assertExists(lookupKey(hz), fetchedServiceAccount)
			Expect(fetchedServiceAccount.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			assertExists(clusterScopedLookupKey(hz), fetchedClusterRoleBinding)

			fetchedService := &corev1.Service{}
			assertExists(lookupKey(hz), fetchedService)
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedSts := &v1.StatefulSet{}
			assertExists(lookupKey(hz), fetchedSts)
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(fetchedCR.DockerImage()))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].ImagePullPolicy).Should(Equal(fetchedCR.Spec.ImagePullPolicy))

			DeleteIfExists(lookupKey(hz), hz)

			By("expecting to ClusterRole and ClusterRoleBinding removed via finalizer")
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRole{})
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRoleBinding{})
		})

		When("applying empty spec", func() {
			emptyHzSpecValues := &test.HazelcastSpecValues{
				ClusterSize:     n.DefaultClusterSize,
				Repository:      n.HazelcastEERepo,
				Version:         n.HazelcastVersion,
				ImagePullPolicy: n.HazelcastImagePullPolicy,
				LicenseKey:      n.LicenseKeySecret,
			}

			It("should create CR with default values", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						LicenseKeySecretName: n.LicenseKeySecret,
					},
				}
				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				ensureSpecEquals(fetchedCR, emptyHzSpecValues)
			})

			It("should update the CR with the default values", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						ClusterSize:          &[]int32{5}[0],
						Repository:           "myorg/hazelcast",
						Version:              "1.0",
						LicenseKeySecretName: "license-key-secret",
						ImagePullPolicy:      corev1.PullAlways,
					},
				}

				licenseSecret := CreateLicenseKeySecret(hz.Spec.GetLicenseKeySecretName(), hz.Namespace)
				assertExists(lookupKey(licenseSecret), licenseSecret)

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				update(fetchedCR, removeSpec)
				fetchedCR = assertHzStatusIsPending(fetchedCR)
				ensureSpecEquals(fetchedCR, emptyHzSpecValues)
			})
		})

		It(fmt.Sprintf("should fail to set cluster size to more than %d", n.ClusterSizeLimit), func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			requestedClusterSize := int32(n.ClusterSizeLimit + 1)
			spec.ClusterSize = &requestedClusterSize

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("Invalid value: 301: may not be greater than 300")))
		})

		It("should fail if CR name is invalid", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "1hz",
					Namespace: namespace,
				},
				Spec: test.HazelcastSpec(defaultHazelcastSpecValues()),
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(HaveOccurred())
		})
	})

	Context("with ExposeExternally configuration", func() {
		It("should create Hazelcast cluster exposed for unisocket client", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			Expect(fetchedCR.Spec.ExposeExternally.Type).Should(Equal(hazelcastv1alpha1.ExposeExternallyTypeUnisocket))
			Expect(fetchedCR.Spec.ExposeExternally.DiscoveryServiceType).Should(Equal(corev1.ServiceTypeNodePort))

			By("checking created services")
			serviceList := fetchServices(hz, 1)

			service := serviceList.Items[0]
			Expect(service.Name).Should(Equal(hz.Name))
			Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
		})

		It("should create Hazelcast cluster exposed for smart client", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			Expect(fetchedCR.Spec.ExposeExternally.Type).Should(Equal(hazelcastv1alpha1.ExposeExternallyTypeSmart))
			Expect(fetchedCR.Spec.ExposeExternally.DiscoveryServiceType).Should(Equal(corev1.ServiceTypeNodePort))
			Expect(fetchedCR.Spec.ExposeExternally.MemberAccess).Should(Equal(hazelcastv1alpha1.MemberAccessNodePortExternalIP))

			By("checking created services")
			serviceList := fetchServices(hz, 4)

			for _, s := range serviceList.Items {
				if s.Name == hz.Name {
					// discovery service
					Expect(s.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
				} else {
					// member access service
					Expect(s.Name).Should(ContainSubstring(hz.Name))
					Expect(s.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
				}
			}
		})

		It("should scale Hazelcast cluster exposed for smart client", func() {
			By("creating the cluster of size 3")
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ClusterSize = &[]int32{3}[0]
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			fetchServices(fetchedCR, 4)

			By("scaling the cluster to 6 members")
			fetchedCR = assertHzStatusIsPending(hz)
			update(fetchedCR, setClusterSize(6))
			fetchedCR = fetchHz(fetchedCR)
			assertHzStatusIsPending(fetchedCR)
			fetchServices(fetchedCR, 7)

			By("scaling the cluster to 1 member")
			fetchedCR = assertHzStatusIsPending(hz)
			update(fetchedCR, setClusterSize(1))
			fetchedCR = fetchHz(fetchedCR)
			assertHzStatusIsPending(fetchedCR)
			fetchServices(fetchedCR, 2)
		})

		It("should allow updating expose externally configuration", func() {
			By("creating the cluster with smart client")
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ClusterSize = &[]int32{3}[0]
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			fetchServices(fetchedCR, 4)

			By("updating type to unisocket")
			fetchedCR = assertHzStatusIsPending(fetchedCR)
			update(fetchedCR, enableUnisocket, disableMemberAccess)

			fetchedCR = fetchHz(fetchedCR)
			assertHzStatusIsPending(fetchedCR)
			fetchServices(fetchedCR, 1)

			By("updating discovery service to LoadBalancer")
			fetchedCR = assertHzStatusIsPending(fetchedCR)
			update(fetchedCR, setDiscoveryViaLoadBalancer)

			fetchedCR = fetchHz(fetchedCR)
			assertHzStatusIsPending(fetchedCR)

			Eventually(func() corev1.ServiceType {
				serviceList := fetchServices(fetchedCR, 1)
				return serviceList.Items[0].Spec.Type
			}).Should(Equal(corev1.ServiceTypeLoadBalancer))

			By("updating type to smart")
			fetchedCR = assertHzStatusIsPending(fetchedCR)
			update(fetchedCR, enableSmart)

			fetchedCR = fetchHz(fetchedCR)
			assertHzStatusIsPending(fetchedCR)
			fetchServices(fetchedCR, 4)

			By("deleting expose externally configuration")
			update(fetchedCR, disableExposeExternally)
			fetchedCR = fetchHz(fetchedCR)
			assertHzStatusIsPending(fetchedCR)
			serviceList := fetchServices(fetchedCR, 1)
			Expect(serviceList.Items[0].Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))
		})

		It("should fail to set MemberAccess for unisocket", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:         hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				MemberAccess: hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("Forbidden: can't be set when exposeExternally.type is set to \"Unisocket\"")))
		})
	})

	Context("with Properties value", func() {
		It("should pass the values to ConfigMap", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			sampleProperties := map[string]string{
				"hazelcast.slow.operation.detector.threshold.millis":           "4000",
				"hazelcast.slow.operation.detector.stacktrace.logging.enabled": "true",
				"hazelcast.query.optimizer.type":                               "NONE",
			}
			samplePropsWithDefaults := make(map[string]string)
			for k, v := range hazelcast.DefaultProperties {
				samplePropsWithDefaults[k] = v
			}
			for k, v := range sampleProperties {
				samplePropsWithDefaults[k] = v
			}
			spec.Properties = sampleProperties
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			_ = assertHzStatusIsPending(hz)

			Eventually(func() map[string]string {
				cfg := getSecret(hz)
				a := &config.HazelcastWrapper{}

				if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
					return nil
				}

				return a.Hazelcast.Properties
			}, timeout, interval).Should(Equal(samplePropsWithDefaults))
		})
	})

	Context("with Scheduling configuration", func() {
		When("NodeSelector is given", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					NodeSelector: map[string]string{
						"node.selector": "1",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() map[string]string {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.NodeSelector
				}, timeout, interval).Should(HaveKeyWithValue("node.selector", "1"))
			})
		})

		When("Affinity is given", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
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
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() *corev1.Affinity {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Affinity
				}, timeout, interval).Should(Equal(spec.Scheduling.Affinity))
			})
		})

		When("Toleration is given", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.zone",
							Operator: corev1.TolerationOpExists,
						},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				Eventually(func() []corev1.Toleration {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Tolerations
				}, timeout, interval).Should(Equal(spec.Scheduling.Tolerations))
			})
		})
	})

	Context("with Image configuration", func() {
		When("ImagePullSecrets are defined", func() {
			It("should pass the values to StatefulSet spec", func() {
				pullSecrets := []corev1.LocalObjectReference{
					{Name: "secret1"},
					{Name: "secret2"},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						LicenseKeySecretName: n.LicenseKeySecret,
						ImagePullSecrets:     pullSecrets,
					},
				}
				create(hz)
				assertHzStatusIsPending(hz)
				fetchedSts := &v1.StatefulSet{}
				assertExists(lookupKey(hz), fetchedSts)
				Expect(fetchedSts.Spec.Template.Spec.ImagePullSecrets).Should(Equal(pullSecrets))
			})
		})
	})

	Context("with HighAvailability configuration", func() {
		When("HighAvailabilityMode is configured as NODE", func() {
			It("should create topologySpreadConstraints", func() {
				s := test.HazelcastSpec(defaultHazelcastSpecValues())
				s.HighAvailabilityMode = "NODE"

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       s,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

				Eventually(func() []corev1.TopologySpreadConstraint {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.TopologySpreadConstraints
				}, timeout, interval).Should(
					ConsistOf(corev1.TopologySpreadConstraint{
						MaxSkew:           1,
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: corev1.ScheduleAnyway,
						LabelSelector:     &metav1.LabelSelector{MatchLabels: labelFilter(hz)},
					}),
				)
			})
		})

		When("HighAvailabilityMode is configured as ZONE", func() {
			It("should create topologySpreadConstraints", func() {
				s := test.HazelcastSpec(defaultHazelcastSpecValues())
				s.HighAvailabilityMode = "ZONE"

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       s,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

				Eventually(func() []corev1.TopologySpreadConstraint {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.TopologySpreadConstraints
				}, timeout, interval).Should(
					ConsistOf(corev1.TopologySpreadConstraint{
						MaxSkew:           1,
						TopologyKey:       "topology.kubernetes.io/zone",
						WhenUnsatisfiable: corev1.ScheduleAnyway,
						LabelSelector:     &metav1.LabelSelector{MatchLabels: labelFilter(hz)},
					}),
				)
			})
		})

		When("HighAvailabilityMode is configured with the scheduling", func() {
			It("should create both of them", func() {
				s := test.HazelcastSpec(defaultHazelcastSpecValues())
				s.HighAvailabilityMode = "ZONE"
				s.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
					Affinity: &corev1.Affinity{
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
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       s,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

				Eventually(func() []corev1.TopologySpreadConstraint {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.TopologySpreadConstraints
				}, timeout, interval).Should(
					ConsistOf(WithTransform(func(tsc corev1.TopologySpreadConstraint) corev1.TopologySpreadConstraint {
						return tsc
					}, Equal(
						corev1.TopologySpreadConstraint{
							MaxSkew:           1,
							TopologyKey:       "topology.kubernetes.io/zone",
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector:     &metav1.LabelSelector{MatchLabels: labelFilter(hz)},
						},
					))),
				)

				ss := getStatefulSet(hz)
				Expect(len(ss.Spec.Template.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution)).To(Equal(1))
			})
		})

		It("should fail to update", func() {
			zoneHASpec := test.HazelcastSpec(defaultHazelcastSpecValues())
			zoneHASpec.HighAvailabilityMode = "ZONE"

			hs, _ := json.Marshal(&zoneHASpec)

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(hs)),
				Spec:       zoneHASpec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			test.CheckHazelcastCR(hz, defaultHazelcastSpecValues())

			var err error
			for {
				Expect(k8sClient.Get(
					context.Background(), types.NamespacedName{Namespace: hz.Namespace, Name: hz.Name}, hz)).Should(Succeed())
				hz.Spec.HighAvailabilityMode = hazelcastv1alpha1.HighAvailabilityNodeMode
				err = k8sClient.Update(context.Background(), hz)
				if errors.IsConflict(err) {
					continue
				}
				break
			}
			Expect(err).Should(MatchError(ContainSubstring("spec.highAvailabilityMode: Forbidden: field cannot be updated")))
		})
	})

	Context("with Persistence configuration", func() {
		It("should create with default values", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

			By("checking the Persistence CR configuration", func() {
				Expect(fetchedCR.Spec.Persistence.ClusterDataRecoveryPolicy).
					Should(Equal(hazelcastv1alpha1.FullRecovery))
				Expect(fetchedCR.Spec.Persistence.PVC.AccessModes).Should(ConsistOf(corev1.ReadWriteOnce))
				Expect(*fetchedCR.Spec.Persistence.PVC.RequestStorage).Should(Equal(resource.MustParse("8Gi")))
			})
		})

		It("should create volumeClaimTemplates", func() {
			s := test.HazelcastSpec(defaultHazelcastSpecValues())
			s.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					StorageClassName: &[]string{"standard"}[0],
				},
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       s,
			}

			create(hz)
			fetchedCR := assertHzStatusIsPending(hz)
			test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

			By("checking the Persistence CR configuration", func() {
				Expect(fetchedCR.Spec.Persistence.ClusterDataRecoveryPolicy).
					Should(Equal(hazelcastv1alpha1.FullRecovery))
				Expect(fetchedCR.Spec.Persistence.PVC.AccessModes).Should(ConsistOf(corev1.ReadWriteOnce))
				Expect(*fetchedCR.Spec.Persistence.PVC.RequestStorage).Should(Equal(resource.MustParse("8Gi")))
				Expect(*fetchedCR.Spec.Persistence.PVC.StorageClassName).Should(Equal("standard"))
			})

			Eventually(func() []corev1.PersistentVolumeClaim {
				ss := getStatefulSet(hz)
				return ss.Spec.VolumeClaimTemplates
			}, timeout, interval).Should(
				ConsistOf(WithTransform(func(pvc corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaimSpec {
					return pvc.Spec
				}, Equal(
					corev1.PersistentVolumeClaimSpec{
						AccessModes: fetchedCR.Spec.Persistence.PVC.AccessModes,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: *fetchedCR.Spec.Persistence.PVC.RequestStorage,
							},
						},
						StorageClassName: fetchedCR.Spec.Persistence.PVC.StorageClassName,
						VolumeMode:       &[]corev1.PersistentVolumeMode{corev1.PersistentVolumeFilesystem}[0],
					},
				))),
			)
		})

		It("should add RBAC PolicyRule for watch StatefulSets", func() {
			s := test.HazelcastSpec(defaultHazelcastSpecValues())
			s.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					StorageClassName: &[]string{"standard"}[0],
				},
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       s,
			}

			create(hz)
			assertHzStatusIsPending(hz)

			By("checking Role", func() {
				rbac := &rbacv1.Role{}
				Expect(k8sClient.Get(
					context.Background(), client.ObjectKey{Name: hz.Name, Namespace: hz.Namespace}, rbac)).
					Should(Succeed())

				Expect(rbac.Rules).Should(ContainElement(rbacv1.PolicyRule{
					APIGroups: []string{"apps"},
					Resources: []string{"statefulsets"},
					Verbs:     []string{"watch", "list"},
				}))
			})
		})

		It("should not create PartialStart with FullRecovery", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
				StartupAction:             hazelcastv1alpha1.PartialStart,
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("PartialStart can be used only with Partial clusterDataRecoveryPolicy")))
		})

		It("should not create if pvc is not specified", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("spec.persistence.pvc: Required value: must be set when persistence is enabled")))
		})

		It("should not create if pvc accessModes is not specified", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("spec.persistence.pvc.accessModes: Required value: must be set when persistence is enabled")))
		})
	})

	Context("with JVM configuration", func() {
		When("Memory is configured", func() {
			It("should set memory with percentages", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				p := pointer.String("10")
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
						InitialRAMPercentage: p,
						MinRAMPercentage:     p,
						MaxRAMPercentage:     p,
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)

				Expect(*fetchedCR.Spec.JVM.Memory.InitialRAMPercentage).Should(Equal(*p))
				Expect(*fetchedCR.Spec.JVM.Memory.MinRAMPercentage).Should(Equal(*p))
				Expect(*fetchedCR.Spec.JVM.Memory.MaxRAMPercentage).Should(Equal(*p))
			})

			It("should set GC params", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				s := hazelcastv1alpha1.GCTypeSerial
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					GC: &hazelcastv1alpha1.JVMGCConfiguration{
						Logging:   pointer.Bool(true),
						Collector: &s,
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)

				Expect(*fetchedCR.Spec.JVM.GC.Logging).Should(Equal(true))
				Expect(*fetchedCR.Spec.JVM.GC.Collector).Should(Equal(s))
			})
		})

		When("incorrect configuration", func() {
			expectedErrStr := `%s is already set up in JVM config"`

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.InitialRamPerArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
						InitialRAMPercentage: pointer.String("10"),
					},
					Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.InitialRamPerArg)},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.InitialRamPerArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.MinRamPerArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
						MinRAMPercentage: pointer.String("10"),
					},
					Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.MinRamPerArg)},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.MinRamPerArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.MaxRamPerArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
						MaxRAMPercentage: pointer.String("10"),
					},
					Args: []string{fmt.Sprintf("%s=10", hazelcastv1alpha1.MaxRamPerArg)},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.MaxRamPerArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.GCLoggingArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					GC: &hazelcastv1alpha1.JVMGCConfiguration{
						Logging: pointer.Bool(true),
					},
					Args: []string{fmt.Sprintf(hazelcastv1alpha1.GCLoggingArg)},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.GCLoggingArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.SerialGCArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				c := hazelcastv1alpha1.GCTypeSerial
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Memory: nil,
					GC: &hazelcastv1alpha1.JVMGCConfiguration{
						Collector: &c,
					},
					Args: []string{fmt.Sprintf(hazelcastv1alpha1.SerialGCArg)},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.SerialGCArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.ParallelGCArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				c := hazelcastv1alpha1.GCTypeParallel
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					GC:   &hazelcastv1alpha1.JVMGCConfiguration{},
					Args: []string{fmt.Sprintf(hazelcastv1alpha1.ParallelGCArg)},
				}
				spec.JVM.GC.Collector = &c

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.ParallelGCArg))))
			})

			It(fmt.Sprintf("should return error if %s configured twice", hazelcastv1alpha1.G1GCArg), func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				c := hazelcastv1alpha1.GCTypeG1
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					GC:   &hazelcastv1alpha1.JVMGCConfiguration{},
					Args: []string{fmt.Sprintf(hazelcastv1alpha1.G1GCArg)},
				}
				spec.JVM.GC.Collector = &c

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring(fmt.Sprintf(expectedErrStr, hazelcastv1alpha1.G1GCArg))))
			})
		})

		When("JVM arg is not configured", func() {
			It("should set the default values for JAVA_OPTS", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)
				ss := getStatefulSet(hz)

				By("Checking if Hazelcast Container has the correct JAVA_OPTS")
				b := strings.Builder{}
				for k, v := range hazelcast.DefaultJavaOptions {
					b.WriteString(fmt.Sprintf(" %s=%s", k, v))
				}
				expectedJavaOpts := b.String()
				javaOpts := ""
				for _, env := range ss.Spec.Template.Spec.Containers[0].Env {
					if env.Name == hazelcast.JavaOpts {
						javaOpts = env.Value
					}
				}
				Expect(javaOpts).To(ContainSubstring(expectedJavaOpts))
			})
		})

		When("JVM args is configured", func() {
			It("should override the default values for JAVA_OPTS", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())

				configuredDefaults := map[string]struct{}{"-Dhazelcast.stale.join.prevention.duration.seconds": {}}
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Args: []string{"-XX:MaxGCPauseMillis=200", "-Dhazelcast.stale.join.prevention.duration.seconds=40"},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)
				ss := getStatefulSet(hz)

				By("Checking if Hazelcast Container has the correct JAVA_OPTS")
				b := strings.Builder{}
				for _, arg := range spec.JVM.Args {
					b.WriteString(fmt.Sprintf(" %s", arg))
				}
				for k, v := range hazelcast.DefaultJavaOptions {
					_, ok := configuredDefaults[k]
					if !ok {
						b.WriteString(fmt.Sprintf(" %s=%s", k, v))
					}
				}
				expectedJavaOpts := b.String()
				javaOpts := ""
				for _, env := range ss.Spec.Template.Spec.Containers[0].Env {
					if env.Name == hazelcast.JavaOpts {
						javaOpts = env.Value
					}
				}

				Expect(javaOpts).To(ContainSubstring(expectedJavaOpts))
			})
		})
	})

	Context("with env variables", func() {
		When("configured", func() {
			It("should set them correctly", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Env = []corev1.EnvVar{
					{
						Name:  "ENV",
						Value: "VAL",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)

				Expect(fetchedCR.Spec.Env[0].Name).Should(Equal("ENV"))
				Expect(fetchedCR.Spec.Env[0].Value).Should(Equal("VAL"))

				ss := getStatefulSet(hz)

				var envs []string
				for _, e := range ss.Spec.Template.Spec.Containers[0].Env {
					envs = append(envs, e.Name)
				}
				Expect(envs).Should(ContainElement("ENV"))
			})
		})
		When("it is configured with env vars starting with HZ_", func() {
			It("should not set them", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Env = []corev1.EnvVar{
					{
						Name:  "HZ_ENV",
						Value: "VAL",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				err := k8sClient.Create(context.Background(), hz)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("Environment variables cannot start with 'HZ_'. Use customConfigCmName to configure Hazelcast."))
			})
		})
		When("it is configured with empty env var name", func() {
			It("should give an error", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Env = []corev1.EnvVar{
					{
						Name:  "",
						Value: "VAL",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				err := k8sClient.Create(context.Background(), hz)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("Environment variable name cannot be empty"))
			})
		})
	})

	Context("with Resources parameters", func() {
		When("resources are given", func() {
			It("should be set to Containers' spec", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
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
				spec.Agent.Resources = &corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("5Gi"),
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)

				for i := 0; i <= 1; i++ {
					Eventually(func() map[corev1.ResourceName]resource.Quantity {
						ss := getStatefulSet(hz)
						return ss.Spec.Template.Spec.Containers[i].Resources.Limits
					}, timeout, interval).Should(And(
						HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")),
						HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("10Gi"))),
					)

					Eventually(func() map[corev1.ResourceName]resource.Quantity {
						ss := getStatefulSet(hz)
						return ss.Spec.Template.Spec.Containers[i].Resources.Requests
					}, timeout, interval).Should(And(
						HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")),
						HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("5Gi"))),
					)
				}
			})
		})
	})

	Context("with SidecarAgent configuration", func() {
		When("Sidecar Agent is configured with Persistence", func() {
			It("should be deployed as a sidecar container", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					PVC: &hazelcastv1alpha1.PvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
						StorageClassName: &[]string{"standard"}[0],
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

				Eventually(func() int {
					ss := getStatefulSet(hz)
					return len(ss.Spec.Template.Spec.Containers)
				}, timeout, interval).Should(Equal(2))
			})
		})
	})

	Context("StatefulSet", func() {
		firstSpec := hazelcastv1alpha1.HazelcastSpec{
			ClusterSize:          pointer.Int32(2),
			Repository:           "hazelcast/hazelcast-enterprise",
			Version:              "5.2",
			ImagePullPolicy:      corev1.PullAlways,
			ImagePullSecrets:     nil,
			ExposeExternally:     nil,
			LicenseKeySecretName: "key-secret",
		}

		secondSpec := hazelcastv1alpha1.HazelcastSpec{
			ClusterSize:     pointer.Int32(3),
			Repository:      "hazelcast/hazelcast",
			Version:         "5.3",
			ImagePullPolicy: corev1.PullIfNotPresent,
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "secret1"},
				{Name: "secret2"},
			},
			ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type: hazelcastv1alpha1.ExposeExternallyTypeSmart,
			},
			LicenseKeySecretName: "license-key",
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
			It("should forward changes to StatefulSet", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       firstSpec,
				}

				licenseSecret := CreateLicenseKeySecret(hz.Spec.GetLicenseKeySecretName(), hz.Namespace)
				assertExists(lookupKey(licenseSecret), licenseSecret)

				create(hz)
				hz = assertHzStatusIsPending(hz)
				hz.Spec = secondSpec

				licenseSecret = CreateLicenseKeySecret(hz.Spec.GetLicenseKeySecretName(), hz.Namespace)
				assertExists(lookupKey(licenseSecret), licenseSecret)

				update(hz)
				ss := getStatefulSet(hz)

				By("Checking if StatefulSet ClusterSize is updated")
				Eventually(func() int32 {
					ss = getStatefulSet(hz)
					return *ss.Spec.Replicas
				}, timeout, interval).Should(Equal(*secondSpec.ClusterSize))

				By("Checking if StatefulSet Image is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].Image).To(Equal(fmt.Sprintf("%s:%s", secondSpec.Repository, secondSpec.Version)))

				By("Checking if StatefulSet ImagePullPolicy is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(secondSpec.ImagePullPolicy))

				By("Checking if StatefulSet ImagePullSecrets is updated")
				Expect(ss.Spec.Template.Spec.ImagePullSecrets).To(Equal(secondSpec.ImagePullSecrets))

				By("Checking if StatefulSet ExposeExternally is updated")
				an, ok := ss.Annotations[n.ServicePerPodCountAnnotation]
				Expect(ok).To(BeTrue())
				Expect(an).To(Equal(strconv.Itoa(int(*hz.Spec.ClusterSize))))

				an, ok = ss.Spec.Template.Annotations[n.ExposeExternallyAnnotation]
				Expect(ok).To(BeTrue())
				Expect(an).To(Equal(string(hz.Spec.ExposeExternally.MemberAccessType())))

				By("Checking if StatefulSet LicenseKeySecretName is updated")
				el := ss.Spec.Template.Spec.Containers[0].Env
				for _, env := range el {
					if env.Name == "HZ_LICENSEKEY" {
						Expect(env.ValueFrom.SecretKeyRef.Key).To(Equal(secondSpec.GetLicenseKeySecretName()))
					}
				}

				By("Checking if StatefulSet Scheduling is updated")
				Expect(*ss.Spec.Template.Spec.Affinity).To(Equal(*secondSpec.Scheduling.Affinity))
				Expect(ss.Spec.Template.Spec.NodeSelector).To(Equal(secondSpec.Scheduling.NodeSelector))
				Expect(ss.Spec.Template.Spec.Tolerations).To(Equal(secondSpec.Scheduling.Tolerations))
				Expect(ss.Spec.Template.Spec.TopologySpreadConstraints).To(Equal(secondSpec.Scheduling.TopologySpreadConstraints))

				By("Checking if StatefulSet Resources is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].Resources).To(Equal(*secondSpec.Resources))
			})
		})
	})

	Context("with UserCodeDeployment configuration", func() {
		When("two Configmaps are given in userCode field", func() {
			It("should put correct fields in StatefulSet", func() {
				cms := []string{
					"cm1",
					"cm2",
				}
				ts := "trigger-sequence"
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						LicenseKeySecretName: n.LicenseKeySecret,
						DeprecatedUserCodeDeployment: &hazelcastv1alpha1.UserCodeDeploymentConfig{
							RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
								ConfigMaps: cms,
							},
							TriggerSequence: ts,
						},
					},
				}

				create(hz)
				hz = assertHzStatusIsPending(hz)
				ss := getStatefulSet(hz)

				By("checking if StatefulSet has the ConfigMap Volumes")
				var expectedVols []corev1.Volume
				for _, cm := range cms {
					expectedVols = append(expectedVols, corev1.Volume{
						Name: n.UserCodeConfigMapNamePrefix + cm + ts,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cm,
								},
								DefaultMode: pointer.Int32(420),
							},
						},
					})
				}
				Expect(ss.Spec.Template.Spec.Volumes).To(ContainElements(expectedVols))

				By("Checking if StatefulSet has the ConfigMap Volumes Mounts")
				var expectedVolMounts []corev1.VolumeMount
				for _, cm := range cms {
					expectedVolMounts = append(expectedVolMounts, corev1.VolumeMount{
						Name:      n.UserCodeConfigMapNamePrefix + cm + ts,
						MountPath: path.Join(n.UserCodeConfigMapPath, cm),
					})
				}
				Expect(ss.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(expectedVolMounts))

				By("Checking if Hazelcast Container has the correct CLASSPATH")
				var b []string

				for _, cm := range cms {
					b = append(b, path.Join(n.UserCodeConfigMapPath, cm, "*"))
				}
				expectedClassPath := strings.Join(b, ":")
				classPath := ""
				for _, env := range ss.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "CLASSPATH" {
						classPath = env.Value
					}
				}
				Expect(classPath).To(ContainSubstring(expectedClassPath))
			})
		})
	})

	Context("LicenseKey", func() {
		When("is given with OS repo", func() {
			It("should mutate EE repo", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						LicenseKeySecretName: "secret-name",
						Repository:           n.HazelcastEERepo,
					},
				}

				licenseSecret := CreateLicenseKeySecret(hz.Spec.GetLicenseKeySecretName(), hz.Namespace)
				assertExists(lookupKey(licenseSecret), licenseSecret)

				create(hz)

				hz = assertHzStatusIsPending(hz)
				Expect(hz.Spec.Repository).Should(Equal(n.HazelcastEERepo))
			})
		})
		When("is not given with EE repo", func() {
			It("should fail", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.LicenseKeySecretName = ""

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("spec.licenseKeySecretName in body should be at least 1 chars long")))
			})
		})
	})

	Context("with AdvancedNetwork configuration", func() {
		When("full configuration", func() {
			It("should create AdvancedNetwork configuration", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
					MemberServerSocketEndpointConfig: hazelcastv1alpha1.ServerSocketEndpointConfig{
						Interfaces: []string{"10.10.1.*"},
					},
					ClientServerSocketEndpointConfig: hazelcastv1alpha1.ServerSocketEndpointConfig{
						Interfaces: []string{"10.10.3.*"},
					},
					WAN: []hazelcastv1alpha1.WANConfig{
						{
							Port:        5710,
							PortCount:   5,
							ServiceType: corev1.ServiceTypeClusterIP,
							Name:        "tokyo",
						},
						{
							Port:      5720,
							PortCount: 5,
							Name:      "istanbul",
						},
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				p := config.AdvancedNetwork{
					Enabled: true,
					Join: config.Join{
						Kubernetes: config.Kubernetes{
							Enabled:                      pointer.Bool(true),
							ServiceName:                  hz.Name,
							UseNodeNameAsExternalAddress: nil,
							ServicePerPodLabelName:       "hazelcast.com/service-per-pod",
							ServicePerPodLabelValue:      "true",
							ServicePort:                  5702,
						},
					},
					MemberServerSocketEndpointConfig: config.MemberServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      5702,
							PortCount: 1,
						},
						Interfaces: config.Interfaces{
							Enabled:    true,
							Interfaces: []string{"10.10.1.*"},
						},
					},
					ClientServerSocketEndpointConfig: config.ClientServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      5701,
							PortCount: 1,
						},
						Interfaces: config.Interfaces{
							Enabled:    true,
							Interfaces: []string{"10.10.3.*"},
						},
					},
					RestServerSocketEndpointConfig: config.RestServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      8081,
							PortCount: 1,
						},
						EndpointGroups: config.EndpointGroups{
							HealthCheck:  config.EndpointGroup{Enabled: pointer.Bool(true)},
							ClusterWrite: config.EndpointGroup{Enabled: pointer.Bool(true)},
							Persistence:  config.EndpointGroup{Enabled: pointer.Bool(true)},
						},
					},
					WanServerSocketEndpointConfig: map[string]config.WanPort{
						"tokyo": {
							PortAndPortCount: config.PortAndPortCount{
								Port:      5710,
								PortCount: 5,
							},
						},
						"istanbul": {
							PortAndPortCount: config.PortAndPortCount{
								Port:      5720,
								PortCount: 5,
							},
						},
					},
				}

				create(hz)
				assertHzStatusIsPending(hz)

				Eventually(func() config.AdvancedNetwork {
					cfg := getSecret(hz)
					a := &config.HazelcastWrapper{}

					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
						return config.AdvancedNetwork{}
					}

					return a.Hazelcast.AdvancedNetwork
				}, timeout, interval).Should(Equal(p))

				By("checking created services")
				serviceList := &corev1.ServiceList{}
				err := k8sClient.List(context.Background(), serviceList, client.InNamespace(hz.Namespace), labelFilter(hz))
				Expect(err).Should(BeNil())

				for _, s := range serviceList.Items {
					if strings.Contains(s.Name, "tokyo") {
						Expect(true).Should(Equal(s.Spec.Type == corev1.ServiceTypeClusterIP))
					}

					if strings.Contains(s.Name, "istanbul") {
						Expect(true).Should(Equal(s.Spec.Type == corev1.ServiceTypeLoadBalancer))
					}
				}
			})
		})

		When("default configuration", func() {
			It("should create default Advanced Network configuration", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				p := config.AdvancedNetwork{
					Enabled: true,
					Join: config.Join{
						Kubernetes: config.Kubernetes{
							Enabled:                      pointer.Bool(true),
							ServiceName:                  hz.Name,
							UseNodeNameAsExternalAddress: nil,
							ServicePerPodLabelName:       "hazelcast.com/service-per-pod",
							ServicePerPodLabelValue:      "true",
							ServicePort:                  5702,
						},
					},
					MemberServerSocketEndpointConfig: config.MemberServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      5702,
							PortCount: 1,
						},
					},
					ClientServerSocketEndpointConfig: config.ClientServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      5701,
							PortCount: 1,
						},
					},
					RestServerSocketEndpointConfig: config.RestServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      8081,
							PortCount: 1,
						},
						EndpointGroups: config.EndpointGroups{
							HealthCheck:  config.EndpointGroup{Enabled: pointer.Bool(true)},
							ClusterWrite: config.EndpointGroup{Enabled: pointer.Bool(true)},
							Persistence:  config.EndpointGroup{Enabled: pointer.Bool(true)},
						},
					},
					WanServerSocketEndpointConfig: map[string]config.WanPort{
						"default": {
							PortAndPortCount: config.PortAndPortCount{
								Port:      n.WanDefaultPort,
								PortCount: 1,
							},
						},
					},
				}

				create(hz)
				assertHzStatusIsPending(hz)

				Eventually(func() config.AdvancedNetwork {
					cfg := getSecret(hz)
					a := &config.HazelcastWrapper{}
					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
						return config.AdvancedNetwork{}
					}
					return a.Hazelcast.AdvancedNetwork
				}, timeout, interval).Should(Equal(p))
				svcList := &corev1.ServiceList{}
				err := k8sClient.List(context.Background(), svcList, client.InNamespace(hz.Namespace), labelFilter(hz))
				Expect(err).Should(BeNil())

				Expect(len(svcList.Items)).Should(Equal(1)) // just the HZ Discovery Service
			})
		})

		It("should fail to overlap WAN ports with each other", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:      5001,
						PortCount: 3,
					},
					{
						Port:      5002,
						PortCount: 3,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(MatchError(
				ContainSubstring("spec.advancedNetwork.wan: Invalid value: \"5001-5003\": wan ports overlapping with 5002-5004")))
		})

		It("should fail to overlap WAN ports with other sockets", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:      5702,
						PortCount: 3,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("spec.advancedNetwork.wan[0]: Invalid value: \"5702-5704\": wan ports conflicting with one of 5701,5702,8081")))
		})

		It("should fail to set ServiceType to non-existing type value", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:        5702,
						PortCount:   3,
						ServiceType: corev1.ServiceTypeExternalName,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("invalid serviceType value, possible values are ClusterIP and LoadBalancer")))
		})
	})

	Context("with NativeMemory configuration", func() {
		When("Native Memory property is configured", func() {
			It("should be enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.NativeMemory = &hazelcastv1alpha1.NativeMemoryConfiguration{
					AllocatorType: hazelcastv1alpha1.NativeMemoryPooled,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)

				Eventually(func() bool {
					cfg := getSecret(hz)

					config := &config.HazelcastWrapper{}
					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], config); err != nil {
						return false
					}

					return config.Hazelcast.NativeMemory.Enabled
				}, timeout, interval).Should(BeTrue())
			})

			It("should fail if NativeMemory.AllocatorType is not POOLED when persistence is enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.NativeMemory = &hazelcastv1alpha1.NativeMemoryConfiguration{
					AllocatorType: hazelcastv1alpha1.NativeMemoryStandard,
				}
				spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					PVC: &hazelcastv1alpha1.PvcConfiguration{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).Should(HaveOccurred())
			})
		})
	})

	Context("with ManagementCenter configuration", func() {
		When("Management Center property is configured", func() {
			It("should be enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.ManagementCenterConfig = &hazelcastv1alpha1.ManagementCenterConfig{
					ScriptingEnabled:  true,
					ConsoleEnabled:    true,
					DataAccessEnabled: true,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)

				Eventually(func() bool {
					cfg := getSecret(hz)

					config := &config.HazelcastWrapper{}
					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], config); err != nil {
						return false
					}

					mc := config.Hazelcast.ManagementCenter
					return mc.DataAccessEnabled && mc.ScriptingEnabled && mc.ConsoleEnabled
				}, timeout, interval).Should(BeTrue())
			})
		})
	})

	Context("with RBAC Permission updates", func() {
		When("RBAC permissions are overridden by a client", func() {
			It("should override changes with operator ones", func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       test.HazelcastSpec(defaultHazelcastSpecValues()),
				}

				create(hz)
				assertHzStatusIsPending(hz)

				rbac := &rbacv1.Role{}
				Expect(k8sClient.Get(
					context.Background(), client.ObjectKey{Name: hz.Name, Namespace: hz.Namespace}, rbac)).
					Should(Succeed())

				rules := rbac.Rules

				// Update rules with empty rules
				newRules := []rbacv1.PolicyRule{}
				rbac.Rules = newRules
				Expect(k8sClient.Update(context.Background(), rbac)).Should(Succeed())

				// Wait for operator to override the client changes
				Eventually(func() []rbacv1.PolicyRule {
					rbac := &rbacv1.Role{}
					Expect(k8sClient.Get(
						context.Background(), client.ObjectKey{Name: hz.Name, Namespace: hz.Namespace}, rbac)).
						Should(Succeed())

					return rbac.Rules
				}, timeout, interval).Should(Equal(rules))
			})
		})
	})

	Context("with TLS configuration", func() {
		When("TLS property is configured", func() {
			It("should be enabled when secret is valid", func() {
				tlsSecret := CreateTLSSecret("tls-secret", namespace)
				assertExists(lookupKey(tlsSecret), tlsSecret)
				defer DeleteIfExists(lookupKey(tlsSecret), tlsSecret)

				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.TLS = &hazelcastv1alpha1.TLS{
					SecretName: tlsSecret.GetName(),
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				assertHzStatusIsPending(hz)

				Eventually(func() bool {
					configMap := getSecret(hz)

					config := &config.HazelcastWrapper{}
					if err := yaml.Unmarshal(configMap.Data["hazelcast.yaml"], config); err != nil {
						return false
					}

					if enabled := *config.Hazelcast.AdvancedNetwork.ClientServerSocketEndpointConfig.SSL.Enabled; !enabled {
						return enabled
					}

					if enabled := *config.Hazelcast.AdvancedNetwork.MemberServerSocketEndpointConfig.SSL.Enabled; !enabled {
						return enabled
					}

					return true
				}, timeout, interval).Should(BeTrue())
			})

			It("should error when secretName is empty", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.TLS = &hazelcastv1alpha1.TLS{
					SecretName:           "",
					MutualAuthentication: hazelcastv1alpha1.MutualAuthenticationRequired,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).Should(HaveOccurred())
			})

			It("should error when secretName does not exist", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.TLS = &hazelcastv1alpha1.TLS{
					SecretName: "notfound",
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).Should(HaveOccurred())
			})
		})
	})

	Context("Hazelcast Validation Multiple Errors", func() {
		It("should return multiple errors", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: "InvalidServiceType",
				MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
			}
			spec.ClusterSize = pointer.Int32(5000)
			spec.AdvancedNetwork = &hazelcastv1alpha1.AdvancedNetwork{
				WAN: []hazelcastv1alpha1.WANConfig{
					{
						Port:      5701,
						PortCount: 20,
					},
					{
						Port:      5709,
						PortCount: 1,
					},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			err := k8sClient.Create(context.Background(), hz)
			Expect(err).Should(MatchError(
				ContainSubstring("spec.exposeExternally.memberAccess:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.clusterSize:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.advancedNetwork.wan:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.advancedNetwork.wan[0]:")))
			Expect(err).Should(MatchError(
				ContainSubstring("spec.exposeExternally.discoveryServiceType:")))
		})
	})

	Context("Hazelcast Persistence Restore Validation", func() {
		It("should return hot backup cannot be found error", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage: resource.NewQuantity(9*2^20, resource.BinarySI),
				},
				Restore: hazelcastv1alpha1.RestoreConfiguration{
					HotBackupResourceName: "notexist",
				},
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			err := k8sClient.Create(context.Background(), hz)
			Expect(err).Should(MatchError(
				ContainSubstring("There is not hot backup found with name notexist")))
		})
	})

	Context("with JetEngine configuration", func() {
		When("fully configured", func() {
			It("should create jet engine configuration", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JetEngineConfiguration = &hazelcastv1alpha1.JetEngineConfiguration{
					Enabled:               ptr.Bool(true),
					ResourceUploadEnabled: false,
					Instance: &hazelcastv1alpha1.JetInstance{
						CooperativeThreadCount:         ptr.Int32(1),
						FlowControlPeriodMillis:        1,
						BackupCount:                    1,
						ScaleUpDelayMillis:             1,
						LosslessRestartEnabled:         false,
						MaxProcessorAccumulatedRecords: ptr.Int64(1),
					},
					EdgeDefaults: &hazelcastv1alpha1.JetEdgeDefaults{
						QueueSize:               ptr.Int32(1),
						PacketSizeLimit:         ptr.Int32(1),
						ReceiveWindowMultiplier: ptr.Int8(1),
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				expectedJetEngineConfig := config.Jet{
					Enabled:               ptr.Bool(true),
					ResourceUploadEnabled: ptr.Bool(false),
					Instance: config.JetInstance{
						CooperativeThreadCount:         ptr.Int32(1),
						FlowControlPeriodMillis:        ptr.Int32(1),
						BackupCount:                    ptr.Int32(1),
						ScaleUpDelayMillis:             ptr.Int32(1),
						LosslessRestartEnabled:         ptr.Bool(false),
						MaxProcessorAccumulatedRecords: ptr.Int64(1),
					},
					EdgeDefaults: config.EdgeDefaults{
						QueueSize:               ptr.Int32(1),
						PacketSizeLimit:         ptr.Int32(1),
						ReceiveWindowMultiplier: ptr.Int8(1),
					},
				}

				create(hz)
				_ = assertHzStatusIsPending(hz)

				Eventually(func() config.Jet {
					cfg := getSecret(hz)
					a := &config.HazelcastWrapper{}

					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
						return config.Jet{}
					}

					return a.Hazelcast.Jet
				}, timeout, interval).Should(Equal(expectedJetEngineConfig))
			})
		})

		When("Jet is not configured", func() {
			It("should be enabled by default", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				_ = assertHzStatusIsPending(hz)

				Eventually(func() bool {
					cfg := getSecret(hz)
					a := &config.HazelcastWrapper{}

					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
						return false
					}

					return *a.Hazelcast.Jet.Enabled
				}, timeout, interval).Should(BeTrue())
			})
		})

		It("should validate backup count", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.JetEngineConfiguration = &hazelcastv1alpha1.JetEngineConfiguration{
				Enabled: pointer.Bool(true),
				Instance: &hazelcastv1alpha1.JetInstance{
					BackupCount: 7,
				},
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(MatchError(
				ContainSubstring("Invalid value: 7: spec.jet.instance.backupCount in body should be less than or equal to 6")))
		})

		When("LosslessRestart is enabled", func() {
			It("should fail if persistence is not enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JetEngineConfiguration = &hazelcastv1alpha1.JetEngineConfiguration{
					Enabled: pointer.Bool(true),
					Instance: &hazelcastv1alpha1.JetInstance{
						LosslessRestartEnabled: true,
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("can be enabled only if persistence enabled")))
			})

			It("should be created successfully if persistence is enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					PVC: &hazelcastv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: resource.NewQuantity(9*2^20, resource.BinarySI),
					},
				}
				spec.JetEngineConfiguration = &hazelcastv1alpha1.JetEngineConfiguration{
					Enabled: ptr.Bool(true),
					Instance: &hazelcastv1alpha1.JetInstance{
						LosslessRestartEnabled: true,
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				hz = assertHzStatusIsPending(hz)

				Expect(hz.Spec.JetEngineConfiguration.Instance.LosslessRestartEnabled).Should(BeTrue())
			})
		})

		When("bucketConfig is configured", func() {
			It("should error when secret doesn't exist with the given bucket secretName", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.JetEngineConfiguration = &hazelcastv1alpha1.JetEngineConfiguration{
					Enabled: ptr.Bool(true),
					RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
							BucketURI:  "gs://my-bucket",
							SecretName: "notfound",
						},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("Bucket credentials Secret not found")))
			})
		})

		When("ConfigMaps are given", func() {
			It("should put correct fields in StatefulSet", func() {
				cms := []string{
					"cm1",
					"cm2",
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						LicenseKeySecretName: n.LicenseKeySecret,
						JetEngineConfiguration: &hazelcastv1alpha1.JetEngineConfiguration{
							Enabled:               pointer.Bool(true),
							ResourceUploadEnabled: true,
							RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
								ConfigMaps: cms,
							},
						},
					},
				}

				create(hz)
				hz = assertHzStatusIsPending(hz)
				ss := getStatefulSet(hz)

				By("Checking if StatefulSet has the ConfigMap Volumes")
				var expectedVols []corev1.Volume
				for _, cm := range cms {
					expectedVols = append(expectedVols, corev1.Volume{
						Name: n.JetConfigMapNamePrefix + cm,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cm,
								},
								DefaultMode: pointer.Int32(420),
							},
						},
					})
				}
				Expect(ss.Spec.Template.Spec.Volumes).To(ContainElements(expectedVols))

				By("Checking if StatefulSet has the ConfigMap Volumes Mounts")
				var expectedVolMounts []corev1.VolumeMount
				for _, cm := range cms {
					expectedVolMounts = append(expectedVolMounts, corev1.VolumeMount{
						Name:      n.JetConfigMapNamePrefix + cm,
						MountPath: path.Join(n.JetJobJarsPath, cm),
					})
				}
				Expect(ss.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(expectedVolMounts))
			})
		})

		When("SQL catalogPersistence is enabled", func() {
			It("should fail if Hazelcast persistence is not enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.SQL = &hazelcastv1alpha1.SQL{
					CatalogPersistenceEnabled: true,
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("catalogPersistence requires Hazelcast persistence enabled")))
			})

			It("should be created successfully if Hazelcast persistence is enabled", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					PVC: &hazelcastv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: resource.NewQuantity(9*2^20, resource.BinarySI),
					},
				}
				spec.SQL = &hazelcastv1alpha1.SQL{
					CatalogPersistenceEnabled: true,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				hz = assertHzStatusIsPending(hz)

				Expect(hz.Spec.SQL.CatalogPersistenceEnabled).Should(BeTrue())
			})

			It("should fail to disable catalogPersistence", func() {
				spec := test.HazelcastSpec(defaultHazelcastSpecValues())
				spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					PVC: &hazelcastv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: resource.NewQuantity(9*2^20, resource.BinarySI),
					},
				}
				spec.SQL = &hazelcastv1alpha1.SQL{
					CatalogPersistenceEnabled: true,
				}

				hzSpec, _ := json.Marshal(&spec)

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(hzSpec)),
					Spec:       spec,
				}

				create(hz)
				hz = assertHzStatusIsPending(hz)
				Expect(hz.Spec.SQL.CatalogPersistenceEnabled).Should(BeTrue())

				hz.Spec.SQL.CatalogPersistenceEnabled = false
				err := k8sClient.Update(context.Background(), hz)
				Expect(err).Should(MatchError(ContainSubstring("field cannot be disabled after it has been enabled")))
			})
		})

	})

	Context("with labels and annotations", func() {
		It("should set labels and annotations to sub-resources", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.Annotations = map[string]string{
				"annotation-example": "hazelcast",
			}
			spec.Labels = map[string]string{
				// user label
				"label-example": "hazelcast",

				// reserved labels
				n.ApplicationNameLabel:         "user",
				n.ApplicationInstanceNameLabel: "user",
				n.ApplicationManagedByLabel:    "user",
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			create(hz)
			assertHzStatusIsPending(hz)

			resources := map[string]client.Object{
				"Secret":         &corev1.Secret{},
				"Service":        &corev1.Service{},
				"ServiceAccount": &corev1.ServiceAccount{},
				"Role":           &rbacv1.Role{},
				"RoleBinding":    &rbacv1.RoleBinding{},
				"StatefulSet":    &v1.StatefulSet{},
			}
			for name, r := range resources {
				assertExists(lookupKey(hz), r)

				// make sure user annotations and labels are present
				Expect(r.GetAnnotations()).To(HaveKeyWithValue("annotation-example", "hazelcast"), name)
				Expect(r.GetLabels()).To(HaveKeyWithValue("label-example", "hazelcast"), name)

				// make sure operator overwrites selector labels
				Expect(r.GetLabels()).To(Not(HaveKeyWithValue(n.ApplicationNameLabel, "user")), name)
				Expect(r.GetLabels()).To(Not(HaveKeyWithValue(n.ApplicationInstanceNameLabel, "user")), name)
				Expect(r.GetLabels()).To(Not(HaveKeyWithValue(n.ApplicationManagedByLabel, "user")), name)
			}

			clusterResources := map[string]client.Object{
				"ClusterRole":        &rbacv1.ClusterRole{},
				"ClusterRoleBinding": &rbacv1.ClusterRoleBinding{},
			}
			for name, r := range clusterResources {
				// we use clusterScopedLookupKey() here and not lookupKey()!
				assertExists(clusterScopedLookupKey(hz), r)

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

	Context("with Tiered Storage configuration", func() {
		When("LocalDevices is configured", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			nativeMemory := &hazelcastv1alpha1.NativeMemoryConfiguration{
				AllocatorType: hazelcastv1alpha1.NativeMemoryPooled,
			}
			spec.NativeMemory = nativeMemory
			localDevices := []hazelcastv1alpha1.LocalDeviceConfig{{
				Name: "local-device-test",
				PVC: &hazelcastv1alpha1.PvcConfiguration{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			}}
			spec.LocalDevices = localDevices

			It("should fail if NativeMemory is not enabled when Tiered Storage is enabled", Label("fast"), func() {
				spec.NativeMemory = nil
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("Required value: Native Memory must be enabled at Hazelcast when Tiered Storage is enabled")))
			})

			It("should fail if pvc is not specified", Label("fast"), func() {
				spec.LocalDevices = []hazelcastv1alpha1.LocalDeviceConfig{{
					Name: "local-device-test",
				}}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("spec.localDevices.pvc: Required value: must be set when LocalDevice is defined")))
			})

			It("should create with default values", Label("fast"), func() {
				spec.NativeMemory = nativeMemory
				spec.LocalDevices = localDevices
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

				By("checking the Local Device configuration", func() {
					localDevice := fetchedCR.Spec.LocalDevices[0]
					Expect(localDevice.Name).Should(Equal("local-device-test"))
					Expect(localDevice.BlockSize).Should(Equal(pointer.Int32(4096)))
					Expect(localDevice.ReadIOThreadCount).Should(Equal(pointer.Int32(4)))
					Expect(localDevice.WriteIOThreadCount).Should(Equal(pointer.Int32(4)))
					Expect(localDevice.PVC.AccessModes).Should(ConsistOf(corev1.ReadWriteOnce))
					Expect(*localDevice.PVC.RequestStorage).Should(Equal(resource.MustParse("8Gi")))
				})

				expectedLocalDeviceConfig := config.LocalDevice{
					BaseDir: path.Join(n.TieredStorageBaseDir, "local-device-test"),
					Capacity: config.Size{
						Value: 8589934592,
						Unit:  "BYTES",
					},
					BlockSize:          pointer.Int32(4096),
					ReadIOThreadCount:  pointer.Int32(4),
					WriteIOThreadCount: pointer.Int32(4),
				}

				Eventually(func() config.LocalDevice {
					cfg := getSecret(hz)
					a := &config.HazelcastWrapper{}

					if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
						return config.LocalDevice{}
					}

					return a.Hazelcast.LocalDevice["local-device-test"]
				}, timeout, interval).Should(Equal(expectedLocalDeviceConfig))
			})

			It("should create volumeClaimTemplates", Label("fast"), func() {
				spec.LocalDevices = []hazelcastv1alpha1.LocalDeviceConfig{{
					Name:               "local-device-test",
					BlockSize:          pointer.Int32(2048),
					ReadIOThreadCount:  pointer.Int32(2),
					WriteIOThreadCount: pointer.Int32(2),
					PVC: &hazelcastv1alpha1.PvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("128G")}[0],
						StorageClassName: &[]string{"standard"}[0],
					},
				}}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				create(hz)
				fetchedCR := assertHzStatusIsPending(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHazelcastSpecValues())

				By("checking the Local Device configuration", func() {
					localDevice := fetchedCR.Spec.LocalDevices[0]
					Expect(localDevice.Name).Should(Equal("local-device-test"))
					Expect(localDevice.BlockSize).Should(Equal(pointer.Int32(2048)))
					Expect(localDevice.ReadIOThreadCount).Should(Equal(pointer.Int32(2)))
					Expect(localDevice.WriteIOThreadCount).Should(Equal(pointer.Int32(2)))
					Expect(localDevice.PVC.AccessModes).Should(ConsistOf(corev1.ReadWriteMany))
					Expect(*localDevice.PVC.RequestStorage).Should(Equal(resource.MustParse("128G")))
					Expect(*localDevice.PVC.StorageClassName).Should(Equal("standard"))

				})
				Eventually(func() []corev1.PersistentVolumeClaim {
					ss := getStatefulSet(hz)
					return ss.Spec.VolumeClaimTemplates
				}, timeout, interval).Should(
					ConsistOf(WithTransform(func(pvc corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaimSpec {
						return pvc.Spec
					}, Equal(
						corev1.PersistentVolumeClaimSpec{
							AccessModes: fetchedCR.Spec.LocalDevices[0].PVC.AccessModes,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: *fetchedCR.Spec.LocalDevices[0].PVC.RequestStorage,
								},
							},
							StorageClassName: fetchedCR.Spec.LocalDevices[0].PVC.StorageClassName,
							VolumeMode:       &[]corev1.PersistentVolumeMode{corev1.PersistentVolumeFilesystem}[0],
						},
					))),
				)
			})
		})
	})

	Context("CP subsystem", func() {
		It("should configure CP subsystem with PVC", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.HazelcastSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					ClusterSize:          pointer.Int32(5),
					CPSubsystem: &hazelcastv1alpha1.CPSubsystem{
						PVC: &hazelcastv1alpha1.PvcConfiguration{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						},
					},
				},
			}
			create(hz)
			assertHzStatusIsPending(hz)
			fetchedSts := &v1.StatefulSet{}
			assertExists(lookupKey(hz), fetchedSts)

			Expect(fetchedSts.Spec.VolumeClaimTemplates).Should(test.ContainVolumeClaimTemplate(n.CPPersistenceVolumeName))
			hzContainer := fetchedSts.Spec.Template.Spec.Containers[0]
			Expect(hzContainer.VolumeMounts).Should(test.ContainVolumeMount(n.CPPersistenceVolumeName, n.CPBaseDir))

			Eventually(func() config.CPSubsystem {
				cfg := getSecret(hz)
				a := &config.HazelcastWrapper{}

				if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
					return config.CPSubsystem{}
				}

				return a.Hazelcast.CPSubsystem
			}, timeout, interval).Should(Equal(config.CPSubsystem{
				CPMemberCount:      5,
				GroupSize:          pointer.Int32(5),
				BaseDir:            n.CPBaseDir,
				PersistenceEnabled: true,
			}))
		})
		It("should configure CP subsystem with Persistence PVC", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec: hazelcastv1alpha1.HazelcastSpec{
					LicenseKeySecretName: n.LicenseKeySecret,
					ClusterSize:          pointer.Int32(5),
					Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
						PVC: &hazelcastv1alpha1.PvcConfiguration{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						},
					},
					CPSubsystem: &hazelcastv1alpha1.CPSubsystem{},
				},
			}
			create(hz)
			assertHzStatusIsPending(hz)
			fetchedSts := &v1.StatefulSet{}
			assertExists(lookupKey(hz), fetchedSts)

			Expect(fetchedSts.Spec.VolumeClaimTemplates).Should(test.ContainVolumeClaimTemplate(n.PVCName))
			hzContainer := fetchedSts.Spec.Template.Spec.Containers[0]
			Expect(hzContainer.VolumeMounts).Should(test.ContainVolumeMount(n.PVCName, n.PersistenceMountPath))
			Expect(hzContainer.VolumeMounts).Should(Not(test.ContainVolumeMount(n.CPPersistenceVolumeName, n.CPBaseDir)))

			Eventually(func() config.CPSubsystem {
				cfg := getSecret(hz)
				a := &config.HazelcastWrapper{}

				if err := yaml.Unmarshal(cfg.Data["hazelcast.yaml"], a); err != nil {
					return config.CPSubsystem{}
				}

				return a.Hazelcast.CPSubsystem
			}, timeout, interval).Should(Equal(config.CPSubsystem{
				CPMemberCount:      5,
				GroupSize:          pointer.Int32(5),
				BaseDir:            n.PersistenceMountPath + n.CPDirSuffix,
				PersistenceEnabled: true,
			}))
		})
	})

	Context("with CP Subsystem configuration", func() {
		It("should not allow no PVC configuration", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.CPSubsystem = &hazelcastv1alpha1.CPSubsystem{}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("PVC should be configured")))
		})
		It("DataLoadTimeoutSeconds cannot be zero", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.CPSubsystem = &hazelcastv1alpha1.CPSubsystem{
				DataLoadTimeoutSeconds: pointer.Int32(0),
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("spec.cpSubsystem.dataLoadTimeoutSeconds in body should be greater than or equal to 1")))
		})
		It("Session TTL must be greater than session heartbeat interval", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.CPSubsystem = &hazelcastv1alpha1.CPSubsystem{
				SessionTTLSeconds:               pointer.Int32(3),
				SessionHeartbeatIntervalSeconds: pointer.Int32(5),
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("must be greater than sessionHeartbeatIntervalSeconds")))
		})

		It("Session TTL must be smaller than or equal to missing CP member auto-removal seconds", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.CPSubsystem = &hazelcastv1alpha1.CPSubsystem{
				SessionTTLSeconds:                 pointer.Int32(10),
				MissingCpMemberAutoRemovalSeconds: pointer.Int32(5),
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("must be smaller than or equal to missingCpMemberAutoRemovalSeconds")))
		})

		It("Should not allow member count less than 3", func() {
			spec := test.HazelcastSpec(defaultHazelcastSpecValues())
			spec.ClusterSize = pointer.Int32(2)
			spec.CPSubsystem = &hazelcastv1alpha1.CPSubsystem{}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("cluster with CP Subsystem enabled can have 3, 5, or 7 members")))
		})
	})
})
