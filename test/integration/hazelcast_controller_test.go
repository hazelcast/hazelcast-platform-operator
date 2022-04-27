package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
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

	SetLicenseKey := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		hz.Spec.LicenseKeySecret = n.LicenseKeySecret
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

	Delete := func(hz *hazelcastv1alpha1.Hazelcast) {
		By("expecting to delete CR successfully")
		fetchedCR := &hazelcastv1alpha1.Hazelcast{}
		deleteIfExists(lookupKey(hz), fetchedCR)
		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey(hz), fetchedCR)
	}

	EnsureStatus := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		By("ensuring that the status is correct")
		Eventually(func() hazelcastv1alpha1.Phase {
			hz = Fetch(hz)
			return hz.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.Pending))
		return hz
	}

	EnsureFailedStatus := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		By("ensuring that the status is failed")
		Eventually(func() hazelcastv1alpha1.Phase {
			hz = Fetch(hz)
			return hz.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.Failed))
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

	Context("Hazelcast CustomResource with default specs", func() {
		It("should handle CR and sub resources correctly", func() {
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
				Controller:         pointer.BoolPtr(true),
				BlockOwnerDeletion: pointer.BoolPtr(true),
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

			By("Expecting to ClusterRole and ClusterRoleBinding removed via finalizer")
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRole{})
			assertDoesNotExist(clusterScopedLookupKey(hz), &rbacv1.ClusterRoleBinding{})

		})
	})

	Context("Hazelcast CustomResource with expose externally", func() {
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

		It("should create Hazelcast cluster exposed for unisocket client", func() {
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

		It("should create Hazelcast cluster exposed for smart client", func() {
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
		It("should scale Hazelcast cluster exposed for smart client", func() {
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

		It("should allow updating expose externally configuration", func() {
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

		It("should return expected messages when exposeExternally is misconfigured", func() {
			By("creating the cluster with unisocket client with incorrect configuration")
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ClusterSize = &[]int32{3}[0]
			spec.ExposeExternally = &hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
				Spec:       spec,
			}

			Create(hz)
			fetchedCR := EnsureFailedStatus(hz)
			Expect(fetchedCR.Status.Message).To(Equal("error validating new Spec: when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set"))

			By("fixing the incorrect configuration")
			Update(fetchedCR, DisableMemberAccess)
			fetchedCR = EnsureStatus(fetchedCR)
			Expect(fetchedCR.Status.Message).To(BeEmpty())

			Delete(fetchedCR)
		})
	})
	Context("Hazelcast CustomResource with default values", func() {
		defaultHzSpecs := &test.HazelcastSpecValues{
			ClusterSize:     n.DefaultClusterSize,
			Repository:      n.HazelcastRepo,
			Version:         n.HazelcastVersion,
			LicenseKey:      "",
			ImagePullPolicy: n.HazelcastImagePullPolicy,
		}
		It("should create CR with default values when empty specs are applied", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: GetRandomObjectMeta(),
			}
			Create(hz)
			fetchedCR := EnsureStatus(hz)
			EnsureSpecEquals(fetchedCR, defaultHzSpecs)
			Delete(hz)
		})
		It("should update the CR with the default values when updating the empty specs are applied", func() {
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

	Context("Hazelcast license validation", func() {
		When("EE repository is used", func() {
			It("should raise error if no license key secret is provided", func() {
				if !ee {
					Skip("This test will only run in EE configuration")
				}

				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.LicenseKeySecret = ""
				hz := &hazelcastv1alpha1.Hazelcast{
					ObjectMeta: GetRandomObjectMeta(),
					Spec:       spec,
				}
				Create(hz)
				fetchedCR := EnsureFailedStatus(hz)
				Expect(fetchedCR.Status.Message).To(Equal("error validating new Spec: when Hazelcast Enterprise is deployed, licenseKeySecret must be set"))

				By("filling the licenseSecretKey should fix it")
				Update(fetchedCR, SetLicenseKey)
				fetchedCR = EnsureStatus(fetchedCR)
				Expect(fetchedCR.Status.Message).To(BeEmpty())

				Delete(fetchedCR)
			})
		})
	})

	Context("Pod scheduling parameters", func() {
		When("NodeSelector is used", func() {
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
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
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
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
			It("should pass the values to StatefulSet spec", func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
				spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
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
	Context("Hazelcast Image configuration", func() {
		When("ImagePullSecrets are defined", func() {
			It("should pass the values to StatefulSet spec", func() {
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

	Context("Hot Restart Persistence configuration", func() {
		When("Persistence is configured", func() {
			It("should create volumeClaimTemplates", func() {
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

				By("Checking the Persistence CR configuration", func() {
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
		})
	})

	Context("Map CR configuration", func() {
		When("Using empty configuration", func() {
			It("should fail to create", func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: GetRandomObjectMeta(),
				}
				By("failing to create Map CR")
				Expect(k8sClient.Create(context.Background(), m)).ShouldNot(Succeed())

			})
		})
		When("Using default configuration", func() {
			It("should create Map CR with default configurations", func() {
				m := &hazelcastv1alpha1.Map{
					ObjectMeta: GetRandomObjectMeta(),
					Spec: hazelcastv1alpha1.MapSpec{
						HazelcastResourceName: "hazelcast",
					},
				}
				By("creating Map CR successfully")
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				ms := m.Spec

				By("checking the CR values with default ones")
				Expect(ms.Name).To(Equal(""))
				Expect(*ms.BackupCount).To(Equal(n.DefaultMapBackupCount))
				Expect(*ms.TimeToLiveSeconds).To(Equal(n.DefaultMapTimeToLiveSeconds))
				Expect(*ms.MaxIdleSeconds).To(Equal(n.DefaultMapMaxIdleSeconds))
				Expect(ms.Eviction.EvictionPolicy).To(Equal(codecTypes.EvictionPolicyType(n.DefaultMapEvictionPolicy)))
				Expect(*ms.Eviction.MaxSize).To(Equal(n.DefaultMapMaxSize))
				Expect(ms.Eviction.MaxSizePolicy).To(Equal(codecTypes.MaxSizePolicyType(n.DefaultMapMaxSizePolicy)))
				Expect(ms.Indexes).To(BeNil())
				Expect(ms.PersistenceEnabled).To(Equal(n.DefaultMapPersistenceEnabled))
				Expect(ms.HazelcastResourceName).To(Equal("hazelcast"))
			})
		})
	})

	Context("Resources context", func() {
		When("Resources are used", func() {
			It("should be set to Container spec", func() {
				spec := test.HazelcastSpec(defaultSpecValues, ee)
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
})
