package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Queue Config", Group("queue"), func() {
	localPort := strconv.Itoa(8500 + GinkgoParallelProcess())

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		Cleanup(context.Background())
		deletePVCs(hzLookupKey)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("Creating Queue configurations", func() {
		It("creates Queue config with correct default values", Tag(Kind|Any), func() {
			setLabelAndCRName("hq-1")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			By("creating the default queue config")
			q := hazelcastconfig.DefaultQueue(qLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
			q = assertDataStructureStatus(qLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Queue{}).(*hazelcastcomv1alpha1.Queue)

			memberConfigXML := memberConfigPortForward(context.Background(), hazelcast, localPort)
			queueConfig := getQueueConfigFromMemberConfig(memberConfigXML, q.GetDSName())
			Expect(queueConfig).NotTo(BeNil())

			Expect(queueConfig.BackupCount).Should(Equal(n.DefaultQueueBackupCount))
			Expect(queueConfig.StatisticsEnabled).Should(Equal(n.DefaultQueueStatisticsEnabled))
			Expect(queueConfig.EmptyQueueTtl).Should(Equal(n.DefaultQueueEmptyQueueTtl))
		})
	})

	Context("Updating Queue configurations", func() {
		It("verifies that Queue Config updates are prohibited", Tag(Kind|Any), func() {
			setLabelAndCRName("hq-2")
			hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
			CreateHazelcastCR(hazelcast)

			By("creating the queue config")
			qs := hazelcastcomv1alpha1.QueueSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzLookupKey.Name,
					BackupCount:           pointer.Int32(3),
				},
				EmptyQueueTtlSeconds: pointer.Int32(10),
				MaxSize:              100,
			}
			q := hazelcastconfig.Queue(qs, qLookupKey, labels)
			Expect(k8sClient.Create(context.Background(), q)).Should(Succeed())
			q = assertDataStructureStatus(qLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, &hazelcastcomv1alpha1.Queue{}).(*hazelcastcomv1alpha1.Queue)

			By("failing to update queue config")
			q.Spec.BackupCount = pointer.Int32(5)
			q.Spec.EmptyQueueTtlSeconds = pointer.Int32(20)
			Expect(k8sClient.Update(context.Background(), q)).
				Should(MatchError(ContainSubstring("spec: Forbidden: cannot be updated")))
		})
	})
})
