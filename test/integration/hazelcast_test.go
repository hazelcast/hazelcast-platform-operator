package integration

import (
	"context"
	"encoding/json"
	"fmt"
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
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"

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

	defaultHzSpecValues := defaultHazelcastSpecValues()

	emptyHzSpecValues := &test.HazelcastSpecValues{
		ClusterSize:     n.DefaultClusterSize,
		Repository:      n.HazelcastRepo,
		Version:         n.HazelcastVersion,
		ImagePullPolicy: n.HazelcastImagePullPolicy,
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

	Context("with default configuration", func() {
		It("should handle CR and sub resources correctly", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHzSpecValues, ee),
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			test.CheckHazelcastCR(fetchedCR, defaultHzSpecValues, ee)

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

			deleteResource(lookupKey(hz), hz)

			By("expecting to ClusterRole and ClusterRoleBinding removed via finalizer")
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRole{})
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRoleBinding{})
		})

		When("using empty spec", func() {
			It("should create CR with default values", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
				}
				Create(hz)
				fetchedCR := EnsureStatus(hz)
				EnsureSpecEquals(fetchedCR, emptyHzSpecValues)
				deleteResource(lookupKey(hz), hz)
			})
		})

		It("should update the CR with the default values when updating the empty specs are applied", Label("fast"), func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
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
			EnsureSpecEquals(fetchedCR, emptyHzSpecValues)
			deleteResource(lookupKey(hz), hz)
		})

		It(fmt.Sprintf("should fail to set cluster size to more than %d", n.ClusterSizeLimit), Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			requestedClusterSize := int32(n.ClusterSizeLimit + 1)
			spec.ClusterSize = &requestedClusterSize

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("cluster size limit is exceeded")))
		})
	})

	Context("with ExposeExternally configuration", func() {
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
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
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

			deleteResource(lookupKey(hz), hz)
		})

		It("should create Hazelcast cluster exposed for smart client", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
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

			deleteResource(lookupKey(hz), hz)
		})

		It("should scale Hazelcast cluster exposed for smart client", Label("fast"), func() {
			By("creating the cluster of size 3")
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
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
			deleteResource(lookupKey(hz), hz)
		})

		It("should allow updating expose externally configuration", Label("fast"), func() {
			By("creating the cluster with smart client")
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
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
			deleteResource(lookupKey(hz), hz)
		})

		It("should fail to set MemberAccess for unisocket", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:         hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				MemberAccess: hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set")))
		})
	})

	Context("with Properties value", func() {
		It("should pass the values to ConfigMap", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			sampleProperties := map[string]string{
				"hazelcast.slow.operation.detector.threshold.millis":           "4000",
				"hazelcast.slow.operation.detector.stacktrace.logging.enabled": "true",
				"hazelcast.query.optimizer.type":                               "NONE",
			}
			spec.Properties = sampleProperties
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
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

			deleteResource(lookupKey(hz), hz)
		})
	})

	Context("with Scheduling configuration", func() {
		When("NodeSelector is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
					NodeSelector: map[string]string{
						"node.selector": "1",
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Create(hz)

				Eventually(func() map[string]string {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.NodeSelector
				}, timeout, interval).Should(HaveKeyWithValue("node.selector", "1"))

				deleteResource(lookupKey(hz), hz)
			})
		})

		When("Affinity is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
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
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Create(hz)

				Eventually(func() *corev1.Affinity {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Affinity
				}, timeout, interval).Should(Equal(spec.Scheduling.Affinity))

				deleteResource(lookupKey(hz), hz)
			})
		})

		When("Toleration is used", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				spec.Scheduling = hazelcastv1alpha1.SchedulingConfiguration{
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
				Create(hz)

				Eventually(func() []corev1.Toleration {
					ss := getStatefulSet(hz)
					return ss.Spec.Template.Spec.Tolerations
				}, timeout, interval).Should(Equal(spec.Scheduling.Tolerations))

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with Image configuration", func() {
		When("ImagePullSecrets are defined", func() {
			It("should pass the values to StatefulSet spec", Label("fast"), func() {
				pullSecrets := []corev1.LocalObjectReference{
					{Name: "secret1"},
					{Name: "secret2"},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec: hazelcastv1alpha1.HazelcastSpec{
						ImagePullSecrets: pullSecrets,
					},
				}
				Create(hz)
				EnsureStatus(hz)
				fetchedSts := &v1.StatefulSet{}
				assertExists(lookupKey(hz), fetchedSts)
				Expect(fetchedSts.Spec.Template.Spec.ImagePullSecrets).Should(Equal(pullSecrets))
				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with JetEngine configuration", func() {
		When("Jet is not configured", func() {
			It("should be enabled by default", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with HighAvailability configuration", func() {
		When("HighAvailabilityMode is configured as NODE", func() {
			It("should create topologySpreadConstraints", Label("fast"), func() {
				s := test.HazelcastSpec(defaultHzSpecValues, ee)
				s.HighAvailabilityMode = "NODE"

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHzSpecValues, ee)

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
				deleteResource(lookupKey(hz), hz)
			})
		})

		When("HighAvailabilityMode is configured as ZONE", func() {
			It("should create topologySpreadConstraints", Label("fast"), func() {
				s := test.HazelcastSpec(defaultHzSpecValues, ee)
				s.HighAvailabilityMode = "ZONE"

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHzSpecValues, ee)

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
				deleteResource(lookupKey(hz), hz)
			})
		})

		When("HighAvailabilityMode is configured with the scheduling", func() {
			It("should create both of them", Label("fast"), func() {
				s := test.HazelcastSpec(defaultHzSpecValues, ee)
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
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       s,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHzSpecValues, ee)

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

				deleteResource(lookupKey(hz), hz)
			})
		})

		It("should fail to update", Label("fast"), func() {
			zoneHASpec := test.HazelcastSpec(defaultHzSpecValues, ee)
			zoneHASpec.HighAvailabilityMode = "ZONE"

			hs, _ := json.Marshal(&zoneHASpec)

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace, n.LastSuccessfulSpecAnnotation, string(hs)),
				Spec:       zoneHASpec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			test.CheckHazelcastCR(hz, defaultHzSpecValues, ee)

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
			Expect(err).
				Should(MatchError(ContainSubstring("highAvailabilityMode cannot be updated")))

			deleteIfExists(lookupKey(hz), hz)
			assertDoesNotExist(lookupKey(hz), hz)
		})
	})

	Context("with Persistence configuration", func() {
		It("should create volumeClaimTemplates", Label("fast"), func() {
			s := test.HazelcastSpec(defaultHzSpecValues, ee)
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
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       s,
			}

			Create(hz)
			fetchedCR := EnsureStatus(hz)
			test.CheckHazelcastCR(fetchedCR, defaultHzSpecValues, ee)

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
			deleteResource(lookupKey(hz), hz)
		})

		It("should add RBAC PolicyRule for watch StatefulSets", Label("fast"), func() {
			s := test.HazelcastSpec(defaultHzSpecValues, ee)
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
				ObjectMeta: randomObjectMeta(namespace),
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

		It("should not create PartialStart with FullRecovery", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir:                   "/baseDir/",
				ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
				StartupAction:             hazelcastv1alpha1.PartialStart,
				Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("startupAction PartialStart can be used only with Partial* clusterDataRecoveryPolicy")))
		})

		It("should not create if pvc is specified", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.Persistence = &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir: "/baseDir/",
			}

			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}

			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("when persistence is enabled \"pvc\" field must be set")))
		})
	})

	Context("with JVM configuration", func() {
		When("Memory is configured", func() {
			It("should set memory with percentages", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				p := ptr.String("10")
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

				Create(hz)
				fetchedCR := EnsureStatus(hz)

				Expect(*fetchedCR.Spec.JVM.Memory.InitialRAMPercentage).Should(Equal(*p))
				Expect(*fetchedCR.Spec.JVM.Memory.MinRAMPercentage).Should(Equal(*p))
				Expect(*fetchedCR.Spec.JVM.Memory.MaxRAMPercentage).Should(Equal(*p))

				deleteResource(lookupKey(hz), hz)
			})
			It("should set GC params", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				s := hazelcastv1alpha1.GCTypeSerial
				gcArg := "-XX:MaxGCPauseMillis=200"
				spec.JVM = &hazelcastv1alpha1.JVMConfiguration{
					GC: &hazelcastv1alpha1.JVMGCConfiguration{
						Logging:   ptr.Bool(true),
						Collector: &s,
						Args:      []string{gcArg},
					},
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)

				Expect(*fetchedCR.Spec.JVM.GC.Logging).Should(Equal(true))
				Expect(*fetchedCR.Spec.JVM.GC.Collector).Should(Equal(s))
				Expect(fetchedCR.Spec.JVM.GC.Args[0]).Should(Equal(gcArg))

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with Resources configuration", func() {
		When("resources are used", func() {
			It("should be set to Container spec", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
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
					ObjectMeta: randomObjectMeta(namespace),
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

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with SidecarAgent configuration", func() {
		When("Sidecar Agent is configured with Persistence", func() {
			It("should be deployed as a sidecar container", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
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
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}

				Create(hz)
				fetchedCR := EnsureStatus(hz)
				test.CheckHazelcastCR(fetchedCR, defaultHzSpecValues, ee)

				Eventually(func() int {
					ss := getStatefulSet(hz)
					return len(ss.Spec.Template.Spec.Containers)
				}, timeout, interval).Should(Equal(2))

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("StatefulSet", func() {
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

		When("updating", func() {
			It("should forward changes to StatefulSet", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with UserCodeDeployment configuration", func() {
		When("two Configmaps are given in userCode field", func() {
			It("should put correct fields in StatefulSet", Label("fast"), func() {
				cms := []string{
					"cm1",
					"cm2",
				}
				ts := "trigger-sequence"
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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
				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("LicenseKey", func() {
		When("is given with OS repo", func() {
			It("should mutate EE repo", Label("fast"), func() {
				h := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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
		When("is not given with EE repo", func() {
			It("should fail", Label("fast"), func() {
				if !ee {
					Skip("This test will only run in EE configuration")
				}
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				spec.LicenseKeySecret = ""

				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
					Spec:       spec,
				}
				Expect(k8sClient.Create(context.Background(), hz)).
					Should(MatchError(ContainSubstring("when Hazelcast Enterprise is deployed, licenseKeySecret must be set")))
			})
		})
	})

	Context("with AdvancedNetwork configuration", func() {
		When("full configuration", func() {
			It("should create Advanced Network configuration", Label("fast"), func() {
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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

		It("should fail to overlap each other", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.AdvancedNetwork = hazelcastv1alpha1.AdvancedNetwork{
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
				ContainSubstring("wan replications ports are overlapping, please check and re-apply")))
		})

		It("should fail to overlap with other sockets", Label("fast"), func() {
			spec := test.HazelcastSpec(defaultHzSpecValues, ee)
			spec.AdvancedNetwork = hazelcastv1alpha1.AdvancedNetwork{
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
				Should(MatchError(ContainSubstring("following port numbers are not in use for wan replication")))
		})
	})

	Context("with NativeMemory configuration", func() {
		When("Native Memory property is configured", func() {
			It("should be enabled", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				spec.NativeMemory = &hazelcastv1alpha1.NativeMemoryConfiguration{
					AllocatorType: hazelcastv1alpha1.NativeMemoryPooled,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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

				deleteResource(lookupKey(hz), hz)
			})
		})
	})

	Context("with ManagementCenter configuration", func() {
		When("Management Center property is configured", func() {
			It("should be enabled", Label("fast"), func() {
				spec := test.HazelcastSpec(defaultHzSpecValues, ee)
				spec.ManagementCenterConfig = hazelcastv1alpha1.ManagementCenterConfig{
					ScriptingEnabled:  true,
					ConsoleEnabled:    true,
					DataAccessEnabled: true,
				}
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: randomObjectMeta(namespace),
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

				deleteResource(lookupKey(hz), hz)
			})
		})
	})
})
