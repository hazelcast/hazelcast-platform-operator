package e2e

import (
	"context"
	. "time"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Hazelcast", Label("hz_multi_namespace"), func() {
	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}

		if deployNamespace != "" {
			DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, deployNamespace, labels)
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)

		deletePVCs(hzLookupKey)

		if deployNamespace != "" {
			tmp := hzLookupKey
			tmp.Namespace = deployNamespace
			assertDoesNotExist(tmp, &hazelcastcomv1alpha1.Hazelcast{})
		}
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})

		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create HZ cluster with custom name and update HZ ready members status", Label("slow"), func() {
		if deployNamespace != "" {
			setCRNamespace(deployNamespace)
		}

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

	Describe("Hazelcast CR dependent CRs", func() {
		When("Hazelcast CR is deleted", func() {
			It("dependent Data Structures and HotBackup CRs should be deleted", Label("fast"), func() {
				if deployNamespace != "" {
					setCRNamespace(deployNamespace)
				}

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
})
