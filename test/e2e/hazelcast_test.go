package e2e

import (
	"context"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

const (
	logInterval = 10 * Millisecond
)

var _ = Describe("Hazelcast", Label("hz"), func() {
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

	It("should create Hazelcast cluster with custom name", Label("fast"), func() {
		setLabelAndCRName("h-1")
		hazelcast := hazelcastconfig.ClusterName(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		assertMemberLogs(hazelcast, "Cluster name: "+hazelcast.Spec.ClusterName)
		evaluateReadyMembers(hzLookupKey)
		assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

		By("removing pods so that cluster gets recreated", func() {
			deletePods(hzLookupKey)
			evaluateReadyMembers(hzLookupKey)
		})
	})

	Context("Hazelcast member status", func() {
		It("should update HZ ready members status", Label("fast"), func() {
			setLabelAndCRName("h-2")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)
			assertMemberLogs(hazelcast, "Members {size:3, ver:3}")

			By("removing pods so that cluster gets recreated", func() {
				deletePods(hzLookupKey)
				evaluateReadyMembers(hzLookupKey)
			})
		})

		It("should update HZ detailed member status", Label("fast"), func() {
			setLabelAndCRName("h-3")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			memberStateT := func(status hazelcastcomv1alpha1.HazelcastMemberStatus) string {
				return string(status.State)
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

		It("check correct pod names and IPs for Hazelcast members", Label("fast"), func() {
			setLabelAndCRName("h-4")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(context.Background(), hzLookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			By("checking hazelcast members pod name")
			for _, member := range hz.Status.Members {
				Expect(member.PodName).Should(ContainSubstring(hz.Name))
				pod := &corev1.Pod{}
				err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: hz.Namespace, Name: member.PodName}, pod)
				Expect(err).ToNot(HaveOccurred())
				Expect(member.Ip).Should(Equal(pod.Status.PodIP))
			}
		})
	})

	Describe("External API errors", func() {
		assertStatusAndMessageEventually := func(phase hazelcastcomv1alpha1.Phase) {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(context.Background(), hzLookupKey, hz)
				Expect(err).ToNot(HaveOccurred())
				return hz.Status.Phase
			}, 3*Minute, interval).Should(Equal(phase))
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
			It("dependent Data Structures and HotBackup CRs should be deleted", Label("fast"), func() {
				if !ee {
					Skip("This test will only run in EE configuration")
				}
				setLabelAndCRName("h-7")
				clusterSize := int32(3)

				hz := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
				CreateHazelcastCR(hz)
				evaluateReadyMembers(hzLookupKey)

				m := hazelcastconfig.DefaultMap(mapLookupKey, hz.Name, labels)
				Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
				assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

				mm := hazelcastconfig.DefaultMultiMap(mmLookupKey, hz.Name, labels)
				Expect(k8sClient.Create(context.Background(), mm)).Should(Succeed())
				assertDataStructureStatus(mmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.MultiMap{})

				rm := hazelcastconfig.DefaultReplicatedMap(rmLookupKey, hz.Name, labels)
				Expect(k8sClient.Create(context.Background(), rm)).Should(Succeed())
				assertDataStructureStatus(rmLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.ReplicatedMap{})

				topic := hazelcastconfig.DefaultTopic(topicLookupKey, hz.Name, labels)
				Expect(k8sClient.Create(context.Background(), topic)).Should(Succeed())
				assertDataStructureStatus(topicLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Topic{})

				DeleteAllOf(hz, &hazelcastcomv1alpha1.HazelcastList{}, hz.Namespace, labels)

				err := k8sClient.Get(context.Background(), mapLookupKey, m)
				Expect(errors.IsNotFound(err)).To(BeTrue())

				err = k8sClient.Get(context.Background(), topicLookupKey, topic)
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	Describe("Hazelcast CR TLS configuration", func() {
		When("TLS property is configured", func() {
			It("should form a cluster and be able to connect", Label("fast"), func() {
				if !ee {
					Skip("This test will only run in EE configuration")
				}
				setLabelAndCRName("h-6")
				hz := hazelcastconfig.HazelcastTLS(hzLookupKey, ee, labels)

				tlsSecretNn := types.NamespacedName{
					Name:      hz.Spec.TLS.SecretName,
					Namespace: hz.Namespace,
				}
				secret := hazelcastconfig.TLSSecret(tlsSecretNn, map[string]string{})
				By("creating TLS secret", func() {
					Eventually(func() error {
						return k8sClient.Create(context.Background(), secret)
					}, Minute, interval).Should(Succeed())
					assertExists(tlsSecretNn, &corev1.Secret{})
				})

				CreateHazelcastCR(hz)
				evaluateReadyMembers(hzLookupKey)
			})
		})

		When("TLS with Mutual Authentication property is configured", func() {
			It("should form a cluster and be able to connect", Label("fast"), func() {
				if !ee {
					Skip("This test will only run in EE configuration")
				}
				setLabelAndCRName("h-7")
				hz := hazelcastconfig.HazelcastMTLS(hzLookupKey, ee, labels)

				tlsSecretNn := types.NamespacedName{
					Name:      hz.Spec.TLS.SecretName,
					Namespace: hz.Namespace,
				}
				secret := hazelcastconfig.TLSSecret(tlsSecretNn, map[string]string{})
				By("creating TLS secret", func() {
					Eventually(func() error {
						return k8sClient.Create(context.Background(), secret)
					}, Minute, interval).Should(Succeed())
					assertExists(tlsSecretNn, &corev1.Secret{})
				})

				CreateHazelcastCR(hz)
				evaluateReadyMembers(hzLookupKey)
			})
		})
	})
})
