package integration

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

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
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Hazelcast controller", func() {
	const (
		namespace = "default"
		finalizer = n.Finalizer

		clusterSize      = n.DefaultClusterSize
		version          = n.HazelcastVersion
		licenseKeySecret = n.LicenseKeySecret
		imagePullPolicy  = n.HazelcastImagePullPolicy
	)

	repository := n.HazelcastRepo
	if ee {
		repository = n.HazelcastEERepo
	}

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

	defaultSpecValues := &test.HazelcastSpecValues{
		ClusterSize:     clusterSize,
		Repository:      repository,
		Version:         version,
		LicenseKey:      licenseKeySecret,
		ImagePullPolicy: imagePullPolicy,
	}

	GetRandomObjectMeta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      fmt.Sprintf("hazelcast-test-%s", uuid.NewUUID()),
			Namespace: namespace,
		}
	}

	Create := func(hz *hazelcastv1alpha1.Hazelcast) {
		By("creating the CR with specs successfully")
		Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
	}

	Fetch := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		By("fetching Hazelcast")
		fetchedCR := &hazelcastv1alpha1.Hazelcast{}
		assertExists(lookupKey(hz), fetchedCR)
		return fetchedCR
	}

	type UpdateFn func(*hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast

	SetClusterSize := func(size int32) UpdateFn {
		return func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
			hz.Spec.ClusterSize = &size
			return hz
		}
	}

	EnableUnisocket := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.Type = hazelcastv1alpha1.ExposeExternallyTypeUnisocket
		hz.Spec.ExposeExternally.MemberAccess = ""
		return hz
	}

	DisableMemberAccess := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.MemberAccess = ""
		return hz
	}

	EnableSmart := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.Type = hazelcastv1alpha1.ExposeExternallyTypeSmart
		return hz
	}

	SetDiscoveryViaLoadBalancer := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally.DiscoveryServiceType = corev1.ServiceTypeLoadBalancer
		return hz
	}

	DisableExposeExternally := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.ExposeExternally = nil
		return hz
	}

	RemoveSpec := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec = hazelcastv1alpha1.HazelcastSpec{}
		return hz
	}

	Update := func(hz *hazelcastv1alpha1.Hazelcast, fns ...UpdateFn) {
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

	Delete := func(obj client.Object) {
		By("expecting to delete CR successfully")
		deleteIfExists(lookupKey(obj), obj)
		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey(obj), obj)
	}

	DeleteWithoutWaiting := func(obj client.Object) {
		By("expecting to delete CR successfully")
		deleteIfExists(lookupKey(obj), obj)
	}

	EnsureStatus := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		By("ensuring that the status is correct")
		Eventually(func() hazelcastv1alpha1.Phase {
			hz = Fetch(hz)
			return hz.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.Pending))
		return hz
	}

	EnsureSpecEquals := func(hz *hazelcastv1alpha1.Hazelcast, other *test.HazelcastSpecValues) *hazelcastv1alpha1.Hazelcast {
		By("ensuring spec is defaulted")
		Eventually(func() *hazelcastv1alpha1.HazelcastSpec {
			hz = Fetch(hz)
			return &hz.Spec
		}, timeout, interval).Should(test.EqualSpecs(other, ee))
		return hz
	}

	// Hazelcast CR configuration and controller tests
	Context("Hazelcast CR with default specs", func() {
		It("should handle CR and sub resources correctly", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       test.HazelcastSpec(defaultSpecValues, ee),
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			test.CheckHazelcastCR(fetchedCR, defaultSpecValues, ee)

			By("ensuring the finalizer added successfully")
			Expect(fetchedCR.Finalizers).To(ContainElement(finalizer))

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

			Delete(hz)

			By("expecting to ClusterRole and ClusterRoleBinding removed via finalizer")
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRole{})
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRoleBinding{})
		})
	})

	Context("Hazelcast CR with expose externally", func() {
		FetchServices := func(hz *hazelcastv1alpha1.Hazelcast, waitForN int) *corev1.ServiceList {
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

		It("should create Hazelcast cluster exposed for unisocket client", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			Expect(fetchedCR.Spec.ExposeExternally.Type).Should(Equal(hazelcastv1alpha1.ExposeExternallyTypeUnisocket))
			Expect(fetchedCR.Spec.ExposeExternally.DiscoveryServiceType).Should(Equal(corev1.ServiceTypeNodePort))

			By("checking created services")
			serviceList := FetchServices(hz, 1)

			service := serviceList.Items[0]
			Expect(service.Name).Should(Equal(hz.Name))
			Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))

			Delete(hz)
		})

		It("should create Hazelcast cluster exposed for smart client", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			Expect(fetchedCR.Spec.ExposeExternally.Type).Should(Equal(hazelcastv1alpha1.ExposeExternallyTypeSmart))
			Expect(fetchedCR.Spec.ExposeExternally.DiscoveryServiceType).Should(Equal(corev1.ServiceTypeNodePort))
			Expect(fetchedCR.Spec.ExposeExternally.MemberAccess).Should(Equal(hazelcastv1alpha1.MemberAccessNodePortExternalIP))

			By("checking created services")
			serviceList := FetchServices(hz, 4)

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

			Delete(fetchedCR)
		})

		It("should scale Hazelcast cluster exposed for smart client", Label("fast"), func() {
			By("creating the cluster of size 3")
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ClusterSize = &[]int32{3}[0]
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			FetchServices(fetchedCR, 4)

			By("scaling the cluster to 6 members")
			fetchedCR = EnsureStatus(hz)
			Update(fetchedCR, SetClusterSize(6))
			fetchedCR = Fetch(fetchedCR)
			EnsureStatus(fetchedCR)
			FetchServices(fetchedCR, 7)

			By("scaling the cluster to 1 member")
			fetchedCR = EnsureStatus(hz)
			Update(fetchedCR, SetClusterSize(1))
			fetchedCR = Fetch(fetchedCR)
			EnsureStatus(fetchedCR)
			FetchServices(fetchedCR, 2)

			By("deleting the cluster")
			Delete(fetchedCR)
		})

		It("should allow updating expose externally configuration", Label("fast"), func() {
			By("creating the cluster with smart client")
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ClusterSize = &[]int32{3}[0]
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			FetchServices(fetchedCR, 4)

			By("updating type to unisocket")
			fetchedCR = EnsureStatus(fetchedCR)
			Update(fetchedCR, EnableUnisocket, DisableMemberAccess)

			fetchedCR = Fetch(fetchedCR)
			EnsureStatus(fetchedCR)
			FetchServices(fetchedCR, 1)

			By("updating discovery service to LoadBalancer")
			fetchedCR = EnsureStatus(fetchedCR)
			Update(fetchedCR, SetDiscoveryViaLoadBalancer)

			fetchedCR = Fetch(fetchedCR)
			EnsureStatus(fetchedCR)

			Eventually(func() corev1.ServiceType {
				serviceList := FetchServices(fetchedCR, 1)
				return serviceList.Items[0].Spec.Type
			}).Should(Equal(corev1.ServiceTypeLoadBalancer))

			By("updating type to smart")
			fetchedCR = EnsureStatus(fetchedCR)
			Update(fetchedCR, EnableSmart)

			fetchedCR = Fetch(fetchedCR)
			EnsureStatus(fetchedCR)
			FetchServices(fetchedCR, 4)

			By("deleting expose externally configuration")
			Update(fetchedCR, DisableExposeExternally)
			fetchedCR = Fetch(fetchedCR)
			EnsureStatus(fetchedCR)
			serviceList := FetchServices(fetchedCR, 1)
			Expect(serviceList.Items[0].Spec.Type).Should(Equal(corev1.ServiceTypeClusterIP))
			Delete(fetchedCR)
		})
	})

	Context("Hazelcast CR with default values", func() {
		defaultHzSpecs := &test.HazelcastSpecValues{
			ClusterSize:     n.DefaultClusterSize,
			Repository:      n.HazelcastRepo,
			Version:         n.HazelcastVersion,
			LicenseKey:      "",
			ImagePullPolicy: n.HazelcastImagePullPolicy,
		}

		It("should create CR with default values when empty specs are applied", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
			}
			Create(hz)
			fetchedCR := EnsureStatus(hz)
			EnsureSpecEquals(fetchedCR, defaultHzSpecs)
			Delete(hz)
		})

		It("should update the CR with the default values when updating the empty specs are applied", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec: hazelcastv1alpha1.HazelcastSpec{
					ClusterSize:      &[]int32{5}[0],
					Repository:       "myorg/hazelcast",
					Version:          "1.0",
					LicenseKeySecret: "licenseKeySecret",
					ImagePullPolicy:  corev1.PullAlways,
				},
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			Update(fetchedCR, RemoveSpec)
			fetchedCR = EnsureStatus(fetchedCR)
			EnsureSpecEquals(fetchedCR, defaultHzSpecs)
			Delete(hz)
		})
	})

	Context("Hazelcast CR with properties", func() {
		It("should pass the values to ConfigMap", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			sampleProperties := map[string]string{
				"hazelcast.slow.operation.detector.threshold.millis":           "4000",
				"hazelcast.slow.operation.detector.stacktrace.logging.enabled": "true",
				"hazelcast.query.optimizer.type":                               "NONE",
			}
			spec.Properties = sampleProperties
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Create(hz)
			_ = EnsureStatus(hz)

			Eventually(func() map[string]string {
				cfg := getConfigMap(hz)
				a := &config.HazelcastWrapper{}

				if err := yaml.Unmarshal([]byte(cfg.Data["hazelcast.yaml"]), a); err != nil {
					return nil
				}

				return a.Hazelcast.Properties
			}, timeout, interval).Should(Equal(sampleProperties))

			Delete(hz)
		})
	})

	Context("Hazelcast CR Pod scheduling parameters", func() {
		When("NodeSelector is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					NodeSelector: map[string]string{
						"node.selector": "1",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(hz)

				Eventually(func() map[string]string {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.NodeSelector
				}, timeout, interval).Should(HaveKeyWithValue("node.selector", "1"))

				Delete(hz)
			})
		})

		When("Affinity is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
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
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(hz)

				Eventually(func() *corev1.Affinity {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Affinity
				}, timeout, interval).Should(Equal(spec.Scheduling.Affinity))

				Delete(hz)
			})
		})

		When("Toleration is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					Tolerations: []corev1.Toleration{
						{
							Key:      "node.zone",
							Operator: corev1.TolerationOpExists,
						},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(hz)

				Eventually(func() []corev1.Toleration {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Tolerations
				}, timeout, interval).Should(Equal(spec.Scheduling.Tolerations))

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR Image configuration", func() {
		When("ImagePullSecrets are defined", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				pullSecrets := []corev1.LocalObjectReference{
					{Name: "secret1"},
					{Name: "secret2"},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						ImagePullSecrets: pullSecrets,
					},
				}
				Create(hz)
				EnsureStatus(hz)
				fetchedSts := &v1.StatefulSet{}
				assertExists(lookupKey(hz), fetchedSts)
				Expect(fetchedSts.Spec.Template.Spec.ImagePullSecrets).Should(Equal(pullSecrets))
				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR Jet Engine configuration", func() {
		When("Jet is not configured", func() {
			It("should be enabled by default", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}

				Create(hz)
				_ = EnsureStatus(hz)

				Eventually(func() bool {
					cfg := getConfigMap(hz)
					a := &config.HazelcastWrapper{}

					if err := yaml.Unmarshal([]byte(cfg.Data["hazelcast.yaml"]), a); err != nil {
						return false
					}

					return *a.Hazelcast.Jet.Enabled
				}, timeout, interval).Should(BeTrue())

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR HighAvailabilityMode Configuration", func() {
		When("HighAvailabilityMode is configured as NODE", func() {
			It("should create topologySpreadConstraints", Label("fast"), func() {
				s := test.HazelcastSpec(defaultSpecValues, ee)
				s.HighAvailabilityMode = "NODE"

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultSpecValues, ee)

				Eventually(func() []corev1.TopologySpreadConstraint {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.TopologySpreadConstraints
				}, timeout, interval).Should(
					ConsistOf(WithTransform(func(tsc corev1.TopologySpreadConstraint) corev1.TopologySpreadConstraint {
						return tsc
					}, Equal(
						corev1.TopologySpreadConstraint{
							MaxSkew:           1,
							TopologyKey:       "kubernetes.io/hostname",
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector:     &metav1.LabelSelector{MatchLabels: labelFilter(hz)},
						},
					))),
				)
				Delete(hz)
			})
		})

		When("HighAvailabilityMode is configured as ZONE", func() {
			It("should create topologySpreadConstraints", Label("fast"), func() {
				s := test.HazelcastSpec(defaultSpecValues, ee)
				s.HighAvailabilityMode = "ZONE"

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultSpecValues, ee)

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
				Delete(hz)
			})
		})

		When("HighAvailabilityMode is configured with the scheduling", func() {
			It("should create both of them", Label("fast"), func() {
				s := test.HazelcastSpec(defaultSpecValues, ee)
				s.HighAvailabilityMode = "ZONE"
				s.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
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
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultSpecValues, ee)

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

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR HotRestart Persistence configuration", func() {
		When("Persistence is configured", func() {
			It("should create volumeClaimTemplates", Label("fast"), func() {
				s := test.HazelcastSpec(defaultSpecValues, ee)
				s.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   "/data/hot-restart/",
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
						StorageClassName: &[]string{"standard"}[0],
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultSpecValues, ee)

				By("checking the Persistence CR configuration", func() {
					Expect(fetchedCR.Spec.Persistence.BaseDir).Should(Equal("/data/hot-restart/"))
					Expect(fetchedCR.Spec.Persistence.ClusterDataRecoveryPolicy).
						Should(Equal(hazelcastv1alpha1.FullRecovery))
					Expect(fetchedCR.Spec.Persistence.Pvc.AccessModes).Should(ConsistOf(corev1.ReadWriteOnce))
					Expect(*fetchedCR.Spec.Persistence.Pvc.RequestStorage).Should(Equal(resource.MustParse("8Gi")))
					Expect(*fetchedCR.Spec.Persistence.Pvc.StorageClassName).Should(Equal("standard"))
				})

				Eventually(func() []corev1.PersistentVolumeClaim {
					ss := getStatefulSet(hz)
					return ss.Spec.VolumeClaimTemplates
				}, timeout, interval).Should(
					ConsistOf(WithTransform(func(pvc corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaimSpec {
						return pvc.Spec
					}, Equal(
						corev1.PersistentVolumeClaimSpec{
							AccessModes: fetchedCR.Spec.Persistence.Pvc.AccessModes,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: *fetchedCR.Spec.Persistence.Pvc.RequestStorage,
								},
							},
							StorageClassName: fetchedCR.Spec.Persistence.Pvc.StorageClassName,
							VolumeMode:       &[]corev1.PersistentVolumeMode{corev1.PersistentVolumeFilesystem}[0],
						},
					))),
				)
				Delete(hz)
			})

			It("should add RBAC PolicyRule for watch statefulsets", Label("fast"), func() {
				s := test.HazelcastSpec(defaultSpecValues, ee)
				s.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   "/data/hot-restart/",
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
						StorageClassName: &[]string{"standard"}[0],
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       s,
				}

				Create(hz)
				EnsureStatus(hz)

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
		})
	})

	Context("Hazelcast CR JVM configuration", func() {
		When("Memory is configured", func() {
			It("should set memory with percentages", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				p := pointer.String("10")
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					Memory: &hazelcastv1alpha1.JVMMemoryConfiguration{
						InitialRAMPercentage: p,
						MinRAMPercentage:     p,
						MaxRAMPercentage:     p,
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)

				Expect(*fetchedCR.Spec.JVM.Memory.InitialRAMPercentage).Should(Equal(*p))
				Expect(*fetchedCR.Spec.JVM.Memory.MinRAMPercentage).Should(Equal(*p))
				Expect(*fetchedCR.Spec.JVM.Memory.MaxRAMPercentage).Should(Equal(*p))

				Delete(hz)
			})
			It("should set GC params", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				s := hazelcastv1alpha1.GCTypeSerial
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					GC: &hazelcastv1alpha1.JVMGCConfiguration{
						Logging:   pointer.Bool(true),
						Collector: &s,
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)

				Expect(*fetchedCR.Spec.JVM.GC.Logging).Should(Equal(true))
				Expect(*fetchedCR.Spec.JVM.GC.Collector).Should(Equal(s))

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR Resources configuration", func() {
		When("Resources are used", func() {
			It("should be set to Container spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
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
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(hz)

				Eventually(func() map[corev1.ResourceName]resource.Quantity {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Containers[0].Resources.Limits
				}, timeout, interval).Should(And(
					HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("500m")),
					HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("10Gi"))),
				)

				Eventually(func() map[corev1.ResourceName]resource.Quantity {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Containers[0].Resources.Requests
				}, timeout, interval).Should(And(
					HaveKeyWithValue(corev1.ResourceCPU, resource.MustParse("250m")),
					HaveKeyWithValue(corev1.ResourceMemory, resource.MustParse("5Gi"))),
				)

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR Sidecar Agent configuration", func() {
		When("Sidecar Agent is configured with Persistence", func() {
			It("should be deployed as a sidecar container", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   "/data/hot-restart/",
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage:   &[]resource.Quantity{resource.MustParse("8Gi")}[0],
						StorageClassName: &[]string{"standard"}[0],
					},
				}

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultSpecValues, ee)

				Eventually(func() int {
					ss := getStatefulSet(hz)
					return len(ss.Spec.Template.Spec.Containers)
				}, timeout, interval).Should(Equal(2))

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR Statefulset updates", func() {
		firstSpec := hazelcastv1alpha1.HazelcastSpec{
			ClusterSize:      pointer.Int32(2),
			Repository:       "hazelcast/hazelcast-enterprise",
			Version:          "5.2",
			ImagePullPolicy:  corev1.PullAlways,
			ImagePullSecrets: nil,
			ExposeExternally: nil,
			LicenseKeySecret: "key-secret",
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
			LicenseKeySecret: "",
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

		When("Hazelcast Spec is updated", func() {
			It("Should forward changes to StatefulSet", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       firstSpec,
				}

				Create(hz)
				hz = EnsureStatus(hz)
				hz.Spec = secondSpec
				Update(hz)
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

				By("Checking if StatefulSet LicenseKeySecret is updated")
				el := ss.Spec.Template.Spec.Containers[0].Env
				for _, env := range el {
					if env.Name == "HZ_LICENSEKEY" {
						Expect(env.ValueFrom.SecretKeyRef.Key).To(Equal(secondSpec.LicenseKeySecret))
					}
				}

				By("Checking if StatefulSet Scheduling is updated")
				Expect(*ss.Spec.Template.Spec.Affinity).To(Equal(*secondSpec.Scheduling.Affinity))
				Expect(ss.Spec.Template.Spec.NodeSelector).To(Equal(secondSpec.Scheduling.NodeSelector))
				Expect(ss.Spec.Template.Spec.Tolerations).To(Equal(secondSpec.Scheduling.Tolerations))
				Expect(ss.Spec.Template.Spec.TopologySpreadConstraints).To(Equal(secondSpec.Scheduling.TopologySpreadConstraints))

				By("Checking if StatefulSet Resources is updated")
				Expect(ss.Spec.Template.Spec.Containers[0].Resources).To(Equal(secondSpec.Resources))

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR User Code Deployment with ConfigMap", func() {
		When("Two Configmaps are given in userCode field", func() {
			It("Should put correct fields in StatefulSet", Label("fast"), func() {
				cms := []string{
					"cm1",
					"cm2",
				}
				ts := "trigger-sequence"
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						UserCodeDeployment: hazelcastv1alpha1.UserCodeDeploymentConfig{
							ConfigMaps:      cms,
							TriggerSequence: ts,
						},
					},
				}

				Create(hz)
				hz = EnsureStatus(hz)
				ss := getStatefulSet(hz)

				By("Checking if StatefulSet has the ConfigMap Volumes")
				expectedVols := []corev1.Volume{}
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
				expectedVolMounts := []corev1.VolumeMount{}
				for _, cm := range cms {
					expectedVolMounts = append(expectedVolMounts, corev1.VolumeMount{
						Name:      n.UserCodeConfigMapNamePrefix + cm + ts,
						MountPath: path.Join(n.UserCodeConfigMapPath, cm),
					})
				}
				Expect(ss.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(expectedVolMounts))

				By("Checking if Hazelcast Container has the correct CLASSPATH")
				b := []string{}

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
				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR mutation", func() {
		When("License key with OS repo is given", func() {
			It("should use EE repo", Label("fast"), func() {
				h := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						LicenseKeySecret: "secret-name",
						Repository:       n.HazelcastRepo,
					},
				}
				Expect(k8sClient.Create(context.Background(), h)).Should(Succeed())
				h = EnsureStatus(h)
				Expect(h.Spec.Repository).Should(Equal(n.HazelcastEERepo))
			})
		})
	})

	Context("Hazelcast CR Advanced Network configuration", func() {
		When("Full Configuration", func() {
			It("should create Advanced Network configuration", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						AdvancedNetwork: hazelcastv1alpha1.AdvancedNetwork{
							MemberServerSocketEndpointConfig: hazelcastv1alpha1.MemberServerSocketEndpointConfig{Interfaces: []string{"10.10.1.*"}},
							WAN: []hazelcastv1alpha1.WANConfig{
								{
									Port:        5710,
									PortCount:   5,
									ServiceType: "NodePort",
									Name:        "tokyo",
								},
							},
						},
					},
				}

				By("creating Hazelcast with Advanced Network Configuration successfully")
				Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())

				p := config.AdvancedNetwork{
					Enabled: true,
					Join: config.Join{
						Kubernetes: config.Kubernetes{
							Enabled:     pointer.Bool(true),
							ServiceName: hz.Name,
							ServicePort: 5702,
						},
					},
					MemberServerSocketEndpointConfig: config.MemberServerSocketEndpointConfig{
						Port: config.PortAndPortCount{
							Port:      5702,
							PortCount: 1,
						},
						Interfaces: config.EnabledAndInterfaces{
							Enabled:    true,
							Interfaces: []string{"10.10.1.*"},
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
						"tokyo": {
							PortAndPortCount: config.PortAndPortCount{
								Port:      5710,
								PortCount: 5,
							},
						},
					},
				}

				Eventually(func() config.AdvancedNetwork {
					cfg := getConfigMap(hz)
					a := &config.HazelcastWrapper{}

					if err := yaml.Unmarshal([]byte(cfg.Data["hazelcast.yaml"]), a); err != nil {
						return config.AdvancedNetwork{}
					}

					return a.Hazelcast.AdvancedNetwork
				}, timeout, interval).Should(Equal(p))

				svcList := &corev1.ServiceList{}
				err := k8sClient.List(context.Background(), svcList, client.InNamespace(hz.Namespace), labelFilter(hz))
				Expect(err).Should(BeNil())

				Expect(len(svcList.Items)).Should(Equal(len(hz.Spec.AdvancedNetwork.WAN)))
			})
		})
	})

	Context("Hazelcast CR Native Memory configuration", func() {
		When("Native Memory property is configured", func() {
			It("should be enabled", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.NativeMemory = &hazelcastv1alpha1.NativeMemoryConfiguration{
					AllocatorType: hazelcastv1alpha1.NativeMemoryPooled,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}

				Create(hz)
				EnsureStatus(hz)

				Eventually(func() bool {
					configMap := getConfigMap(hz)

					config := &config.HazelcastWrapper{}
					if err := yaml.Unmarshal([]byte(configMap.Data["hazelcast.yaml"]), config); err != nil {
						return false
					}

					return config.Hazelcast.NativeMemory.Enabled
				}, timeout, interval).Should(BeTrue())

				Delete(hz)
			})
		})
	})

	Context("Hazelcast CR Management Center configuration", func() {
		When("Management Center property is configured", func() {
			It("should be enabled", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.ManagementCenterConfig = hazelcastv1alpha1.ManagementCenterConfig{
					ScriptingEnabled:  true,
					ConsoleEnabled:    true,
					DataAccessEnabled: true,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}

				Create(hz)
				EnsureStatus(hz)

				Eventually(func() bool {
					configMap := getConfigMap(hz)

					config := &config.HazelcastWrapper{}
					if err := yaml.Unmarshal([]byte(configMap.Data["hazelcast.yaml"]), config); err != nil {
						return false
					}

					mc := config.Hazelcast.ManagementCenter
					return mc.DataAccessEnabled && mc.ScriptingEnabled && mc.ConsoleEnabled
				}, timeout, interval).Should(BeTrue())

				Delete(hz)
			})
		})
	})

	// Map CR configuration and controller tests
	Context("Map CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create Map CR")
				Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())
				Delete(m)

			})
		})
		When("Using default configuration", func() {
			It("should create Map CR with default configurations", Label("fast"), func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.MapSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
					},
				}
				By("creating Map CR successfully")
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				ms := m.Spec

				By("checking the CR values with default ones")
				Expect(ms.Name).To(Equal(""))
				Expect(*ms.BackupCount).To(Equal(n.DefaultMapBackupCount))
				Expect(ms.TimeToLiveSeconds).To(Equal(n.DefaultMapTimeToLiveSeconds))
				Expect(ms.MaxIdleSeconds).To(Equal(n.DefaultMapMaxIdleSeconds))
				Expect(ms.Eviction.EvictionPolicy).To(Equal(hazelcastv1alpha1.EvictionPolicyType(n.DefaultMapEvictionPolicy)))
				Expect(ms.Eviction.MaxSize).To(Equal(n.DefaultMapMaxSize))
				Expect(ms.Eviction.MaxSizePolicy).To(Equal(hazelcastv1alpha1.MaxSizePolicyType(n.DefaultMapMaxSizePolicy)))
				Expect(ms.Indexes).To(BeNil())
				Expect(ms.PersistenceEnabled).To(Equal(n.DefaultMapPersistenceEnabled))
				Expect(ms.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(ms.EntryListeners).To(BeNil())
				Delete(m)
			})
			It("should create Map CR with native memory configuration", Label("fast"), func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.MapSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
						InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
					},
				}
				By("creating Map CR successfully")
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				ms := m.Spec

				By("checking the CR values with native memory")
				Expect(ms.InMemoryFormat).To(Equal(hazelcastv1alpha1.InMemoryFormatNative))
				Delete(m)
			})
		})
	})

	// MultiMap CR configuration and controller tests
	Context("MultiMap CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				mm := &hazelcastv1alpha1.MultiMap{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create MultiMap CR")
				Expect(k8sClient.Create(context.Background(), mm)).ShouldNot(Succeed())

			})
		})
		When("Using default configuration", func() {
			It("should create MultiMap CR with default configurations", Label("fast"), func() {
				mm := &hazelcastv1alpha1.MultiMap{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.MultiMapSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
					},
				}
				By("creating MultiMap CR successfully")
				Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
				mms := mm.Spec

				By("checking the CR values with default ones")
				Expect(mms.Name).To(Equal(""))
				Expect(*mms.BackupCount).To(Equal(n.DefaultMultiMapBackupCount))
				Expect(mms.Binary).To(Equal(n.DefaultMultiMapBinary))
				Expect(string(mms.CollectionType)).To(Equal(n.DefaultMultiMapCollectionType))
				Expect(mms.HazelcastResourceName).To(Equal("hazelcast"))
			})
		})
	})

	// CronHotBackup CR configuration and controller tests
	Context("CronHotBackup CR configuration", func() {
		When("CronJob With Empty Spec is created", func() {
			It("Should fail to create", Label("fast"), func() {
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create CronHotBackup CR")
				Expect(k8sClient.Create(context.Background(), chb)).ShouldNot(Succeed())
				Delete(chb)
			})
		})

		When("Using default configuration", func() {
			It("should create CronHotBackup with default values", Label("fast"), func() {
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.CronHotBackupSpec{
						Schedule: "* * * * *",
						HotBackupTemplate: hazelcastv1alpha1.HotBackupTemplateSpec{
							Spec: hazelcastv1alpha1.HotBackupSpec{},
						},
					},
				}
				By("creating CronHotBackup CR successfully")
				Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())
				chbs := chb.Spec

				By("checking the CR values with default ones")
				Expect(*chbs.SuccessfulHotBackupsHistoryLimit).To(Equal(n.DefaultSuccessfulHotBackupsHistoryLimit))
				Expect(*chbs.FailedHotBackupsHistoryLimit).To(Equal(n.DefaultFailedHotBackupsHistoryLimit))
				Delete(chb)
			})
		})

		When("Giving labels and annotations to HotBackup Template", func() {
			It("should HotBackup with those labels", Label("fast"), func() {
				ans := map[string]string{
					"annotation1": "val",
					"annotation2": "val2",
				}
				labels := map[string]string{
					"label1": "val",
					"label2": "val2",
				}
				chb := &hazelcastv1alpha1.CronHotBackup{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.CronHotBackupSpec{
						Schedule: "*/1 * * * * *",
						HotBackupTemplate: hazelcastv1alpha1.HotBackupTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: ans,
								Labels:      labels,
							},
							Spec: hazelcastv1alpha1.HotBackupSpec{},
						},
					},
				}
				By("creating CronHotBackup CR successfully")
				Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())

				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: chb.Name, Namespace: chb.Namespace}, chb)).Should(Succeed())
				// Wait for at least two HotBackups to get created
				time.Sleep(2 * time.Second)
				hbl := &hazelcastv1alpha1.HotBackupList{}
				Eventually(func() string {
					err := k8sClient.List(context.Background(), hbl, client.InNamespace(namespace), client.MatchingLabels(labels))
					if err != nil {
						return ""
					}
					if len(hbl.Items) < 1 {
						return ""
					}
					return hbl.Items[0].Name
				}, timeout, interval).Should(ContainSubstring(chb.Name))

				Expect(hbl.Items[0].Annotations).To(Equal(ans))
				Expect(hbl.Items[0].Labels).To(Equal(labels))

				// Foreground deletion does not work in integration tests, should delete CronHotBackup without waiting
				DeleteWithoutWaiting(chb)
				Expect(k8sClient.DeleteAllOf(context.Background(), &hazelcastv1alpha1.HotBackup{}, client.InNamespace(namespace))).Should(Succeed())
			})
		})
	})

	// Topic CR configuration and controller tests
	Context("Topic CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				t := &hazelcastv1alpha1.Topic{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create Topic CR")
				Expect(k8sClient.Create(context.Background(), t)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create Topic CR with default configurations", Label("fast"), func() {
				t := &hazelcastv1alpha1.Topic{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.TopicSpec{
						HazelcastResourceName: "hazelcast",
					},
				}
				By("creating Topic CR successfully")
				Expect(k8sClient.Create(context.Background(), t)).Should(Succeed())
				ts := t.Spec

				By("checking the CR values with default ones")
				Expect(ts.Name).To(Equal(""))
				Expect(ts.GlobalOrderingEnabled).To(Equal(n.DefaultTopicGlobalOrderingEnabled))
				Expect(ts.MultiThreadingEnabled).To(Equal(n.DefaultTopicMultiThreadingEnabled))
				Expect(ts.HazelcastResourceName).To(Equal("hazelcast"))
			})
		})
	})

	// ReplicatedMap CR configuration and controller tests
	Context("ReplicatedMap CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				t := &hazelcastv1alpha1.ReplicatedMap{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create ReplicatedMap CR")
				Expect(k8sClient.Create(context.Background(), t)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create ReplicatedMap CR with default configurations", Label("fast"), func() {
				rm := &hazelcastv1alpha1.ReplicatedMap{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.ReplicatedMapSpec{
						HazelcastResourceName: "hazelcast",
					},
				}
				By("creating ReplicatedMap CR successfully")
				Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
				rms := rm.Spec

				By("checking the CR values with default ones")
				Expect(rms.Name).To(Equal(""))
				Expect(string(rms.InMemoryFormat)).To(Equal(n.DefaultReplicatedMapInMemoryFormat))
				Expect(*rms.AsyncFillup).To(Equal(n.DefaultReplicatedMapAsyncFillup))
				Expect(rms.HazelcastResourceName).To(Equal("hazelcast"))
			})
		})
	})

	// Queue CR configuration and controller tests
	Context("Queue CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				q := &hazelcastv1alpha1.Queue{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create Queue CR")
				Expect(k8sClient.Create(context.Background(), q)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create Queue CR with default configurations", Label("fast"), func() {
				q := &hazelcastv1alpha1.Queue{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.QueueSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
					},
				}
				By("creating Queue CR successfully")
				Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
				qs := q.Spec

				By("checking the CR values with default ones")
				Expect(qs.Name).To(Equal(""))
				Expect(*qs.BackupCount).To(Equal(n.DefaultQueueBackupCount))
				Expect(qs.AsyncBackupCount).To(Equal(n.DefaultQueueAsyncBackupCount))
				Expect(qs.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(*qs.EmptyQueueTtlSeconds).To(Equal(n.DefaultQueueEmptyQueueTtl))
				Expect(qs.MaxSize).To(Equal(n.DefaultMapMaxSize))
				Expect(qs.PriorityComparatorClassName).To(Equal(""))
			})
		})
	})

	// Cache CR configuration and controller tests
	Context("Cache CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create Cache CR")
				Expect(k8sClient.Create(context.Background(), q)).ShouldNot(Succeed())
			})
		})

		When("Using default configuration", func() {
			It("should create Cache CR with default configurations", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.CacheSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
						InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
					},
				}
				By("creating Cache CR successfully")
				Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
				qs := q.Spec

				By("checking the CR values with default ones")
				Expect(qs.Name).To(Equal(""))
				Expect(*qs.BackupCount).To(Equal(n.DefaultCacheBackupCount))
				Expect(qs.AsyncBackupCount).To(Equal(n.DefaultCacheAsyncBackupCount))
				Expect(qs.HazelcastResourceName).To(Equal("hazelcast"))
				Expect(qs.KeyType).To(Equal(""))
				Expect(qs.ValueType).To(Equal(""))
			})

			It("should create Cache CR with native memory configuration", Label("fast"), func() {
				q := &hazelcastv1alpha1.Cache{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.CacheSpec{
						DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
							HazelcastResourceName: "hazelcast",
						},
						InMemoryFormat: hazelcastv1alpha1.InMemoryFormatNative,
					},
				}
				By("creating Cache CR successfully")
				Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
				qs := q.Spec

				By("checking the CR values with native memory")
				Expect(qs.InMemoryFormat).To(Equal(hazelcastv1alpha1.InMemoryFormatNative))
			})
		})
	})

	// WanReplication CR configuration and controller tests
	Context("WanReplication CR configuration", func() {
		When("endpoints are configured without port", func() {
			It("should set default port to endpoints", Label("fast"), func() {
				endpoints := "10.0.0.1,10.0.0.2,10.0.0.3"
				wan := &hazelcastv1alpha1.WanReplication{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.WanReplicationSpec{
						Resources: []hazelcastv1alpha1.ResourceSpec{
							{
								Name: "hazelcast",
								Kind: hazelcastv1alpha1.ResourceKindHZ,
							},
						},
						TargetClusterName: "dev",
						Endpoints:         endpoints,
					},
				}
				By("creating WanReplication CR successfully")
				Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
				Eventually(func() string {
					newWan := hazelcastv1alpha1.WanReplication{}
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: wan.Name, Namespace: wan.Namespace}, &newWan)
					if err != nil {
						return ""
					}
					return newWan.Spec.Endpoints
				}, timeout, interval).Should(Equal("10.0.0.1:5701,10.0.0.2:5701,10.0.0.3:5701"))
			})
		})
	})

	// WanReplication CR configuration and controller tests
	Context("Hazelcast RBAC Permission updates", func() {
		When("RBAC permissions are overridden by a client", func() {
			It("should override changes with operator ones", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       test.HazelcastSpec(defaultSpecValues, ee),
				}

				Create(hz)
				EnsureStatus(hz)

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
})
