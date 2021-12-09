package integration

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Hazelcast controller", func() {
	const (
		hzKeyName = "hazelcast-test"
		namespace = "default"
		finalizer = n.Finalizer

		clusterSize      = n.DefaultClusterSize
		version          = n.HazelcastVersion
		licenseKeySecret = n.LicenseKeySecret
	)

	repository := n.HazelcastRepo
	if ee {
		repository = n.HazelcastEERepo
	}

	lookupKey := types.NamespacedName{
		Name:      hzKeyName,
		Namespace: namespace,
	}

	clusterScopedLookupKey := types.NamespacedName{
		Name:      (&hazelcastv1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{Name: hzKeyName, Namespace: namespace}}).ClusterScopedName(),
		Namespace: "",
	}

	labelFilter := client.MatchingLabels{
		n.ApplicationNameLabel:      n.Hazelcast,
		n.ApplicationManagedByLabel: n.OperatorName,
	}

	defaultSpecValues := &test.HazelcastSpecValues{
		ClusterSize: clusterSize,
		Repository:  repository,
		Version:     version,
		LicenseKey:  licenseKeySecret,
	}

	Create := func(hz *hazelcastv1alpha1.Hazelcast) {
		By("creating the CR with specs successfully")
		Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
	}

	Update := func(hz *hazelcastv1alpha1.Hazelcast) {
		By("updating the CR with specs successfully")
		Expect(k8sClient.Update(context.Background(), hz)).Should(Succeed())
	}

	Fetch := func() *hazelcastv1alpha1.Hazelcast {
		By("fetching Hazelcast")
		fetchedCR := &hazelcastv1alpha1.Hazelcast{}
		assertExists(lookupKey, fetchedCR)
		return fetchedCR
	}

	Delete := func() {
		By("expecting to delete CR successfully")
		fetchedCR := &hazelcastv1alpha1.Hazelcast{}

		deleteIfExists(lookupKey, fetchedCR)

		By("expecting to CR delete finish")
		assertDoesNotExist(lookupKey, fetchedCR)
	}

	EnsureStatus := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		By("ensuring that the status is correct")
		Eventually(func() hazelcastv1alpha1.Phase {
			hz = Fetch()
			return hz.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.Pending))
		return hz

	}

	EnsureFailedStatus := func(hz *hazelcastv1alpha1.Hazelcast) *hazelcastv1alpha1.Hazelcast {
		By("ensuring that the status is failed")
		Eventually(func() hazelcastv1alpha1.Phase {
			hz = Fetch()
			return hz.Status.Phase
		}, timeout, interval).Should(Equal(hazelcastv1alpha1.Failed))
		return hz
	}

	Context("Hazelcast CustomResource with default specs", func() {
		It("should handle CR and sub resources correctly", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: test.HazelcastSpec(defaultSpecValues, ee),
			}

			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)

			fetchedCR = EnsureStatus(fetchedCR)
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
			assertExists(clusterScopedLookupKey, fetchedClusterRole)

			fetchedServiceAccount := &corev1.ServiceAccount{}
			assertExists(lookupKey, fetchedServiceAccount)
			Expect(fetchedServiceAccount.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			assertExists(clusterScopedLookupKey, fetchedClusterRoleBinding)

			fetchedService := &corev1.Service{}
			assertExists(lookupKey, fetchedService)
			Expect(fetchedService.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

			fetchedSts := &v1.StatefulSet{}
			assertExists(lookupKey, fetchedSts)
			Expect(fetchedSts.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			Expect(fetchedSts.Spec.Template.Spec.Containers[0].Image).Should(Equal(fetchedCR.DockerImage()))

			Delete()

			By("Expecting to ClusterRole removed via finalizer")
			assertDoesNotExist(clusterScopedLookupKey, &rbacv1.ClusterRole{})
		})
	})

	Context("Hazelcast CustomResource with expose externally", func() {
		FetchServices := func(waitForN int) *corev1.ServiceList {
			serviceList := &corev1.ServiceList{}
			Eventually(func() bool {
				err := k8sClient.List(context.Background(), serviceList, client.InNamespace(namespace), labelFilter)
				if err != nil || len(serviceList.Items) != waitForN {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			return serviceList
		}

		It("should create Hazelcast cluster exposed for unisocket client", func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ExposeExternally = hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: spec,
			}
			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)
			fetchedCR = EnsureStatus(fetchedCR)
			Expect(fetchedCR.Spec.ExposeExternally.Type).Should(Equal(hazelcastv1alpha1.ExposeExternallyTypeUnisocket))
			Expect(fetchedCR.Spec.ExposeExternally.DiscoveryServiceType).Should(Equal(corev1.ServiceTypeNodePort))

			By("checking created services")
			serviceList := FetchServices(1)

			service := serviceList.Items[0]
			Expect(service.Name).Should(Equal(hz.Name))
			Expect(service.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))

			Delete()
		})

		It("should create Hazelcast cluster exposed for smart client", func() {
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ExposeExternally = hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: spec,
			}
			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)
			fetchedCR = EnsureStatus(fetchedCR)
			Expect(fetchedCR.Spec.ExposeExternally.Type).Should(Equal(hazelcastv1alpha1.ExposeExternallyTypeSmart))
			Expect(fetchedCR.Spec.ExposeExternally.DiscoveryServiceType).Should(Equal(corev1.ServiceTypeNodePort))
			Expect(fetchedCR.Spec.ExposeExternally.MemberAccess).Should(Equal(hazelcastv1alpha1.MemberAccessNodePortExternalIP))

			By("checking created services")
			serviceList := FetchServices(4)

			for _, s := range serviceList.Items {
				if s.Name == lookupKey.Name {
					// discovery service
					Expect(s.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
				} else {
					// member access service
					Expect(s.Name).Should(ContainSubstring(lookupKey.Name))
					Expect(s.Spec.Type).Should(Equal(corev1.ServiceTypeNodePort))
				}
			}

			Delete()
		})
		It("should scale Hazelcast cluster exposed for smart client", func() {
			By("creating the cluster of size 3")
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ClusterSize = 3
			spec.ExposeExternally = hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: spec,
			}
			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)
			FetchServices(4)

			By("scaling the cluster to 6 members")
			fetchedCR = EnsureStatus(hz)
			fetchedCR.Spec.ClusterSize = 6
			Update(fetchedCR)
			FetchServices(7)

			By("scaling the cluster to 1 member")
			fetchedCR = Fetch()
			fetchedCR.Spec.ClusterSize = 1
			Update(fetchedCR)
			FetchServices(2)

			By("deleting the cluster")
			Delete()
		})

		It("should allow updating expose externally configuration", func() {
			By("creating the cluster with smart client")
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ClusterSize = 3
			spec.ExposeExternally = hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: spec,
			}
			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)
			FetchServices(4)

			By("updating type to unisocket")
			fetchedCR = EnsureStatus(hz)
			fetchedCR.Spec.ExposeExternally.Type = hazelcastv1alpha1.ExposeExternallyTypeUnisocket
			fetchedCR.Spec.ExposeExternally.MemberAccess = ""
			Update(fetchedCR)
			FetchServices(1)

			By("updating discovery service to LoadBalancer")
			fetchedCR = Fetch()
			fetchedCR.Spec.ExposeExternally.DiscoveryServiceType = corev1.ServiceTypeLoadBalancer
			Update(fetchedCR)
			Eventually(func() corev1.ServiceType {
				serviceList := FetchServices(1)
				return serviceList.Items[0].Spec.Type
			}, timeout, interval).Should(Equal(corev1.ServiceTypeLoadBalancer))

			By("updating type to smart")
			fetchedCR = Fetch()
			fetchedCR.Spec.ExposeExternally.Type = hazelcastv1alpha1.ExposeExternallyTypeSmart
			Update(fetchedCR)
			FetchServices(4)

			By("deleting expose externally configuration")
			fetchedCR = Fetch()
			fetchedCR.Spec.ExposeExternally = hazelcastv1alpha1.ExposeExternallyConfiguration{}
			Update(fetchedCR)
			Eventually(func() corev1.ServiceType {
				serviceList := FetchServices(1)
				return serviceList.Items[0].Spec.Type
			}, timeout, interval).Should(Equal(corev1.ServiceTypeClusterIP))

			Delete()
		})

		It("should return expected messages when exposeExternally is misconfigured", func() {
			By("creating the cluster with unisocket client with incorrect configuration")
			spec := test.HazelcastSpec(defaultSpecValues, ee)
			spec.ClusterSize = 3
			spec.ExposeExternally = hazelcastv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: corev1.ServiceTypeNodePort,
				MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: spec,
			}

			var fetchedCR *hazelcastv1alpha1.Hazelcast

			Create(hz)
			fetchedCR = EnsureFailedStatus(fetchedCR)
			Expect(fetchedCR.Status.Message).To(Equal("error validating new Spec: when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set"))

			By("fixing the incorrect configuration")
			fetchedCR.Spec.ExposeExternally.MemberAccess = ""
			Update(fetchedCR)
			fetchedCR = EnsureStatus(fetchedCR)
			Expect(fetchedCR.Status.Message).To(BeEmpty())

			Delete()
		})
	})
	Context("Hazelcast CustomResource with default values", func() {
		defaultHzSpecs := hazelcastv1alpha1.HazelcastSpec{
			ClusterSize:      n.DefaultClusterSize,
			Repository:       n.HazelcastRepo,
			Version:          n.HazelcastVersion,
			LicenseKeySecret: "",
		}
		It("should create CR with default values when empty specs are applied", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
			}

			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)
			fetchedCR = EnsureStatus(fetchedCR)
			Expect(fetchedCR.Spec).To(Equal(defaultHzSpecs))

			Delete()
		})
		It("should update the CR with the default values when updating the empty specs are applied", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lookupKey.Name,
					Namespace: lookupKey.Namespace,
				},
				Spec: hazelcastv1alpha1.HazelcastSpec{
					ClusterSize:      5,
					Repository:       "myorg/hazelcast",
					Version:          "1.0",
					LicenseKeySecret: "licenseKeySecret",
				},
			}
			fetchedCR := &hazelcastv1alpha1.Hazelcast{}

			Create(hz)
			fetchedCR = EnsureStatus(fetchedCR)

			fetchedCR.Spec = hazelcastv1alpha1.HazelcastSpec{}
			Update(fetchedCR)
			Eventually(func() hazelcastv1alpha1.HazelcastSpec {
				fetchedCR = Fetch()
				return fetchedCR.Spec
			}, timeout, interval).Should(Equal(defaultHzSpecs))

			Delete()
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
					ObjectMeta: metav1.ObjectMeta{
						Name:      lookupKey.Name,
						Namespace: lookupKey.Namespace,
					},
					Spec: spec,
				}
				fetchedCR := &hazelcastv1alpha1.Hazelcast{}

				Create(hz)
				fetchedCR = EnsureFailedStatus(fetchedCR)
				Expect(fetchedCR.Status.Message).To(Equal("error validating new Spec: when Hazelcast Enterprise is deployed, licenseKeySecret must be set"))

				By("filling the licenseSecretKey should fix it")
				fetchedCR.Spec.LicenseKeySecret = n.LicenseKeySecret
				Update(fetchedCR)
				fetchedCR = EnsureStatus(fetchedCR)
				Expect(fetchedCR.Status.Message).To(BeEmpty())

				Delete()
			})
		})
	})
})
