package e2e

import (
	"context"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

const (
	logInterval = 10 * Millisecond
)

var _ = Describe("Hazelcast", Label("hz"), func() {
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
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Describe("Default Hazelcast CR", func() {
		It("should create Hazelcast cluster", Label("fast"), func() {
			setLabelAndCRName("h-1")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
		})
	})

	Describe("Hazelcast cluster name", func() {
		It("should create a Hazelcust cluster with Cluster name: development", Label("fast"), func() {
			setLabelAndCRName("h-2")
			hazelcast := hazelcastconfig.ClusterName(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		})
	})

	Context("Hazelcast member status", func() {
		It("should update HZ ready members status", Label("slow"), func() {
			setLabelAndCRName("h-3")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey, 3)
			assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				deletePods(hzLookupKey)
				evaluateReadyMembers(hzLookupKey, 3)
			})
		})

		It("should update HZ detailed member status", Label("fast"), func() {
			setLabelAndCRName("h-4")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey, 3)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			memberStateT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) string {
				return status.State
			}
			masterT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) bool {
				return status.Master
			}
			Eventually(func() []hazelcastcomv1alpha1.HazelcastMemberStatus {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Members
			}, 30*Second, interval).Should(And(HaveLen(3),
				ContainElement(WithTransform(memberStateT, Equal("ACTIVE"))),
				ContainElement(WithTransform(masterT, Equal(true))),
			))
		})
	})

	Describe("External API errors", func() {
		assertStatusAndMessageEventually := func(phase hazelcastcomv1alpha1.Phase) {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Phase
			}, 30*Second, interval).Should(Equal(phase))
			Expect(hz.Status.Message).Should(Not(BeEmpty()))
		}

		It("should be reflected to Hazelcast CR status", Label("fast"), func() {
			setLabelAndCRName("h-5")
			CreateHazelcastCRWithoutCheck(hazelcastconfig.Faulty(hzLookupKey, ee, labels))
			assertStatusAndMessageEventually(hazelcastcomv1alpha1.Failed)
		})
	})

	Describe("Hazelcast CR dependent CRs", func() {
		When("Hazelcast CR is deleted", func() {
			It("dependent Map, and HotBackup CRs should be deleted", Label("fast"), func() {
				if !ee {
					Skip("This test will only run in EE configuration")
				}
				setLabelAndCRName("h-6")
				hz := hazelcastconfig.PersistenceEnabled(hzLookupKey, "/data/hot-backup", labels)
				CreateHazelcastCR(hz)
				evaluateReadyMembers(hzLookupKey, 3)

				m := hazelcastconfig.DefaultMap(mapLookupKey, hz.Name, labels)
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

				hb := hazelcastconfig.HotBackup(hbLookupKey, hz.Name, labels)
				Expect(k8sClient.Create(context.Background(), hb)).Should(Succeed())
				assertHotBackupSuccess(hb, 1*Minute)

				DeleteAllOf(hz, &hazelcastcomv1alpha1.HazelcastList{}, hz.Namespace, labels)

				err := k8sClient.Get(context.Background(), mapLookupKey, m)
				Expect(errors.IsNotFound(err)).To(BeTrue())

				err = k8sClient.Get(context.Background(), hbLookupKey, hb)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})
})
