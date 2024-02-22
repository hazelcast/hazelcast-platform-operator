package e2e

import (
	"context"
	"strconv"
	. "time"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hazelcast Topic Config", Group("topic"), func() {
	localPort := strconv.Itoa(8700 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		Cleanup(context.Background())
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Creating Topic configurations", func() {
		It("creates Topic config with correct default values", Tag(Kind|Any), func() {
			setLabelAndCRName("ht-1")
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
	})

	Context("Updating Topic configurations", func() {
		It("verifies that Topic Config updates are prohibited", Tag(Kind|Any), func() {
			setLabelAndCRName("ht-2")
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
			Expect(k8sClient.Update(context.Background(), topic)).Should(MatchError(ContainSubstring("spec: Forbidden: cannot be updated")))
		})
	})
})
