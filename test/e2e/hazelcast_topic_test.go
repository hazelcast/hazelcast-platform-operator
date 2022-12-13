package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Topic Config", Label("topic"), func() {
	localPort := strconv.Itoa(8700 + GinkgoParallelProcess())

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
		DeleteAllOf(&hazelcastcomv1alpha1.Topic{}, &hazelcastcomv1alpha1.TopicList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should create Topic Config", Label("fast"), func() {
		setLabelAndCRName("ht-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		topic := hazelcastconfig.DefaultTopic(topicLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), topic)).Should(Succeed())
		assertDataStructureStatus(topicLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Topic{})
	})

	It("should create Topic Config with correct default values", Label("fast"), func() {
		setLabelAndCRName("ht-2")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the default topic config")
		topic := hazelcastconfig.DefaultTopic(topicLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), topic)).Should(Succeed())
		topic = assertDataStructureStatus(topicLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Topic{}).(*hazelcastcomv1alpha1.Topic)

		memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
		topicConfig := getTopicConfigFromMemberConfig(memberConfigXML, topic.GetDSName())
		Expect(topicConfig).NotTo(BeNil())

		Expect(topicConfig.GlobalOrderingEnabled).Should(Equal(n.DefaultTopicGlobalOrderingEnabled))
		Expect(topicConfig.MultiThreadingEnabled).Should(Equal(n.DefaultTopicMultiThreadingEnabled))
		Expect(topicConfig.StatisticsEnabled).Should(Equal(n.DefaultTopicStatisticsEnabled))
	})

	It("should fail to update Topic Config", Label("fast"), func() {
		setLabelAndCRName("ht-3")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		By("creating the topic config")
		topics := hazelcastcomv1alpha1.TopicSpec{
			HazelcastResourceName: hzLookupKey.Name,
			GlobalOrderingEnabled: true,
			MultiThreadingEnabled: false,
		}
		topic := hazelcastconfig.Topic(topics, topicLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), topic)).Should(Succeed())
		topic = assertDataStructureStatus(topicLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Topic{}).(*hazelcastcomv1alpha1.Topic)

		By("failing to update topic config")
		topic.Spec.GlobalOrderingEnabled = false
		topic.Spec.MultiThreadingEnabled = true
		Expect(k8sClient.Update(context.Background(), topic)).Should(Succeed())
		assertDataStructureStatus(topicLookupKey, hazelcastcomv1alpha1.DataStructureFailed, &hazelcastcomv1alpha1.Topic{})
	})
})
