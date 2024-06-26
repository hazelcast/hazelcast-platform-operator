package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast JetJobSnapshot", Group("jetjobsnapshot"), func() {
	localPort := strconv.Itoa(9100 + GinkgoParallelProcess())
	jarName := "snapshot-test.jar"

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.JetJobSnapshot{}, &hazelcastcomv1alpha1.JetJobSnapshotList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.JetJob{}, &hazelcastcomv1alpha1.JetJobList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&corev1.Secret{}, &corev1.SecretList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("JetJob snapshot utilization", func() {
		It("should export snapshot and initialize new job from that snapshot", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("jjs-1")

			hazelcast := hazelcastconfig.JetConfigured(hzLookupKey, labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCR(hazelcast)

			By("creating Map CR")
			ctx := context.Background()

			// foo map
			fooMapNn := types.NamespacedName{
				Name:      "foo",
				Namespace: hzLookupKey.Namespace,
			}
			fooMap := hazelcastconfig.MapWithEventJournal(fooMapNn, hazelcast.Name, labels)
			Expect(k8sClient.Create(ctx, fooMap)).Should(Succeed())
			assertMapStatus(fooMap, hazelcastcomv1alpha1.MapSuccess)

			// bar map
			barMapNn := types.NamespacedName{
				Name:      "bar",
				Namespace: hzLookupKey.Namespace,
			}
			barMap := hazelcastconfig.DefaultMap(barMapNn, hazelcast.Name, labels)
			Expect(k8sClient.Create(ctx, barMap)).Should(Succeed())
			assertMapStatus(barMap, hazelcastcomv1alpha1.MapSuccess)

			// fizz map
			fizzMapNn := types.NamespacedName{
				Name:      "fizz",
				Namespace: hzLookupKey.Namespace,
			}
			fizzMap := hazelcastconfig.DefaultMap(fizzMapNn, hazelcast.Name, labels)
			Expect(k8sClient.Create(ctx, fizzMap)).Should(Succeed())
			assertMapStatus(fizzMap, hazelcastcomv1alpha1.MapSuccess)

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(jarName, hzLookupKey.Name, jjLookupKey, labels)
			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/jetJobs",
			}
			jj.Spec.MainClass = "com.hazelcast.operator.test.jobs.FromFooToBarJob"
			Expect(k8sClient.Create(ctx, jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobRunning)

			By("port-forwarding to Hazelcast master pod")
			stopChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
			defer closeChannel(stopChan)

			cl := newHazelcastClientPortForward(ctx, hazelcast, localPort)
			defer func() {
				err := cl.Shutdown(ctx)
				Expect(err).To(BeNil())
			}()

			By("putting entries to map Foo")
			entries := map[string]string{"one": "one", "two": "two", "three": "three"}
			fooHzMap, err := cl.GetMap(ctx, fooMap.MapName())
			Expect(err).To(Not(HaveOccurred()))
			for k, v := range entries {
				_, err := fooHzMap.Put(ctx, k, v)
				Expect(err).To(Not(HaveOccurred()))
			}

			By(fmt.Sprintf("asserting size of map Bar is %d", len(entries)))
			Eventually(func() int {
				barHzMap, err := cl.GetMap(ctx, barMap.MapName())
				if err != nil {
					return 0
				}
				size, err := barHzMap.Size(ctx)
				if err != nil {
					return 0
				}
				return size
			}, Minute, interval).Should(Equal(len(entries)))

			By("creating JetJobSnapshot CR")
			jjs := hazelcastconfig.JetJobSnapshot(jjsLookupKey.Name, false, jj.Name, jjsLookupKey, labels)
			Expect(k8sClient.Create(ctx, jjs)).Should(Succeed())

			By("asserting JetJobSnapshot CR status")
			jjs = checkJetJobSnapshotStatus(jjsLookupKey, hazelcastcomv1alpha1.JetJobSnapshotExported)
			Expect(jjs.Status.CreationTime.IsZero()).To(BeFalse())

			By("ensuring the snapshot is exported on members")
			snapshotMap, err := cl.GetMap(ctx, fmt.Sprintf("__jet.exportedSnapshot.%s", jjs.SnapshotName()))
			Expect(err).NotTo(HaveOccurred())
			Expect(snapshotMap.Size(ctx)).Should(BeNumerically(">", 0))

			By("creating a new JetJob CR initialized from snapshot")
			jjFromSnapshotNn := types.NamespacedName{
				Name:      jjLookupKey.Name + "-from-snapshot",
				Namespace: jjLookupKey.Namespace,
			}
			jjFromSnapshot := hazelcastconfig.JetJobWithInitialSnapshot(jarName, hzLookupKey.Name, jjs.Name, jjFromSnapshotNn, labels)
			jjFromSnapshot.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/jetJobs",
			}
			jjFromSnapshot.Spec.MainClass = "com.hazelcast.operator.test.jobs.FromFooToFizzJob"
			Expect(k8sClient.Create(ctx, jjFromSnapshot)).Should(Succeed())
			checkJetJobStatus(jjFromSnapshotNn, hazelcastcomv1alpha1.JetJobRunning)

			By("asserting size of map Fizz is empty")
			fizzHzMap, err := cl.GetMap(ctx, fizzMap.MapName())
			Expect(err).Should(Not(HaveOccurred()))
			Expect(fizzHzMap.Size(ctx)).Should(BeZero())
		})
	})

	Context("Operational behavior", func() {
		It("cancel the JetJob after successful snapshot export", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("jjs-2")

			hazelcast := hazelcastconfig.JetConfigured(hzLookupKey, labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCR(hazelcast)

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(jarName, hzLookupKey.Name, jjLookupKey, labels)
			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/jetJobs",
			}
			jj.Spec.MainClass = "com.hazelcast.operator.test.jobs.LoggingJob"
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobRunning)

			By("creating JetJobSnapshot CR")
			jjs := hazelcastconfig.JetJobSnapshot(jjsLookupKey.Name, true, jj.Name, jjsLookupKey, labels)
			Expect(k8sClient.Create(context.Background(), jjs)).Should(Succeed())

			jjs = checkJetJobSnapshotStatus(jjsLookupKey, hazelcastcomv1alpha1.JetJobSnapshotExported)
			Expect(jjs.Status.CreationTime.IsZero()).To(BeFalse())

			By("asserting JetJob is canceled")
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobExecutionFailed)
		})

		It("fails when export snapshot from a suspended JetJob", Tag(Kind|AnyCloud), func() {
			setLabelAndCRName("jjs-3")

			hazelcast := hazelcastconfig.JetConfigured(hzLookupKey, labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCR(hazelcast)

			By("creating JetJob CR which is in non-running status")
			jj := hazelcastconfig.JetJob(jarName, hzLookupKey.Name, jjLookupKey, labels)
			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/jetJobs",
			}
			jj.Spec.MainClass = "com.hazelcast.operator.test.jobs.LoggingJob"
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobRunning)

			By("suspending the JetJob")
			Expect(k8sClient.Get(context.Background(), jjLookupKey, jj)).Should(Succeed())
			jj.Spec.State = hazelcastcomv1alpha1.SuspendedJobState
			Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobSuspended)

			By("creating JetJobSnapshot CR")
			jjs := hazelcastconfig.JetJobSnapshot(jjsLookupKey.Name, false, jj.Name, jjsLookupKey, labels)
			Expect(k8sClient.Create(context.Background(), jjs)).Should(Succeed())

			By("asserting JetJobSnapshot status is set to 'failed'")
			checkJetJobSnapshotStatus(jjsLookupKey, hazelcastcomv1alpha1.JetJobSnapshotFailed)
			Eventually(func() string {
				err := k8sClient.Get(context.Background(), jjsLookupKey, jjs)
				if err != nil {
					return ""
				}
				return jjs.Status.Message
			}, 5*Minute, interval).Should(ContainSubstring(
				"JetJob status must be equal to '%s' to be able to export snapshot", hazelcastcomv1alpha1.JetJobRunning))
		})
	})
})
