package e2e

import (
	"context"
	"fmt"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast JetJob", Group("jetjob"), func() {
	fastRunJar := "jet-pipeline-1.0.2.jar"
	longRunJar := "jet-pipeline-longrun-2.0.0.jar"

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.JetJob{}, &hazelcastcomv1alpha1.JetJobList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, &hazelcastcomv1alpha1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&corev1.Secret{}, &corev1.SecretList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("JetJob execution", func() {
		DescribeTable("execute it from different sources", func(secretName, url string) {
			setLabelAndCRName("jj-1")

			var hazelcast *hazelcastcomv1alpha1.Hazelcast
			if secretName != "" {
				hazelcast = hazelcastconfig.JetWithBucketConfigured(hzLookupKey, ee, secretName, url, labels)
			} else {
				hazelcast = hazelcastconfig.JetWithUrlConfigured(hzLookupKey, ee, url, labels)
			}
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCR(hazelcast)

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(fastRunJar, hzLookupKey.Name, jjLookupKey, labels)
			t := Now()
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobCompleted)

			By("Checking the JetJob jar was executed")
			logs := InitLogs(t, hzLookupKey)
			logReader := test.NewLogReader(logs)
			defer logReader.Close()
			test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
				Should(ContainElements(
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 0", jj.JobName())),
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 1", jj.JobName())),
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 13", jj.JobName())),
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 89", jj.JobName()))))
		},
			Entry("using jar from bucket", Tag(Fast|Any), "br-secret-gcp", "gs://operator-user-code/jetJobs"),
			Entry("using jar from remote url", Tag(Fast|Any), "", "https://storage.googleapis.com/operator-user-code-urls-public/jet-pipeline-1.0.2.jar"),
		)

		DescribeTable("download and execute JetJob with external JAR", func(secretName, url string) {
			setLabelAndCRName("jj-2")

			hazelcast := hazelcastconfig.JetConfigured(hzLookupKey, ee, labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCR(hazelcast)

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(fastRunJar, hzLookupKey.Name, jjLookupKey, labels)
			if secretName != "" {
				jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastcomv1alpha1.BucketConfiguration{
					SecretName: secretName,
					BucketURI:  url,
				}
			} else {
				jj.Spec.JetRemoteFileConfiguration.RemoteURL = url
			}
			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/jetJobs",
			}

			t := Now()
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobCompleted)

			By("Checking the JetJob jar was executed")
			logs := InitLogs(t, hzLookupKey)
			logReader := test.NewLogReader(logs)
			defer logReader.Close()
			test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
				Should(ContainElements(
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 0", jj.JobName())),
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 1", jj.JobName())),
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 13", jj.JobName())),
					ContainSubstring(fmt.Sprintf("[%s/loggerSink#0] 89", jj.JobName()))))
		},
			Entry("using jar from bucket", Tag(Fast|Any), "br-secret-gcp", "gs://operator-user-code/jetJobs"),
			Entry("using jar from remote url", Tag(Fast|Any), "", "https://storage.googleapis.com/operator-user-code-urls-public/jet-pipeline-1.0.2.jar"),
		)
	})

	Context("Operational behavior", func() {
		It("verifies status change for a running JetJob", Tag(Fast|Any), func() {
			setLabelAndCRName("jj-3")

			hazelcast := hazelcastconfig.JetWithBucketConfigured(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/jetJobs", labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCR(hazelcast)

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(longRunJar, hzLookupKey.Name, jjLookupKey, labels)
			t := Now()
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobRunning)

			By("Checking the JetJob jar is running")
			logs := InitLogs(t, hzLookupKey)
			logReader := test.NewLogReader(logs)
			defer logReader.Close()
			test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
				Should(ContainElements(
					MatchRegexp(fmt.Sprintf(".*\\[%s\\/\\w+#\\d+\\]\\s+SimpleEvent\\(timestamp=.*,\\s+sequence=\\d+\\).*", jj.Name))))

			Expect(k8sClient.Get(context.Background(), jjLookupKey, jj)).Should(Succeed())
			jj.Spec.State = hazelcastcomv1alpha1.SuspendedJobState
			Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobSuspended)

			Expect(k8sClient.Get(context.Background(), jjLookupKey, jj)).Should(Succeed())
			jj.Spec.State = hazelcastcomv1alpha1.RunningJobState
			Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobRunning)
		})

		It("fails the JetJob if Hazelcast cluster is failing", Tag(Fast|Any), func() {
			setLabelAndCRName("jj-4")

			hazelcast := hazelcastconfig.JetWithBucketConfigured(hzLookupKey, ee, "br-secret-gcp", "gs://wrong-bucket-name/jetJobs", labels)
			hazelcast.Spec.ClusterSize = pointer.Int32(1)
			CreateHazelcastCRWithoutCheck(hazelcast)
			By("checking Hazelcast CR in Pending state", func() {
				hz := &hazelcastcomv1alpha1.Hazelcast{}
				Eventually(func() hazelcastcomv1alpha1.Phase {
					_ = k8sClient.Get(context.Background(), hzLookupKey, hz)
					return hz.Status.Phase
				}, 10*Minute, interval).Should(Equal(hazelcastcomv1alpha1.Pending))
			})

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(fastRunJar, hzLookupKey.Name, jjLookupKey, labels)
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobFailed)

			By("Update Hazelcast cluster with correct configuration")
			UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				hazelcast.Spec.JetEngineConfiguration.RemoteFileConfiguration.BucketConfiguration.BucketURI = "gs://operator-user-code/jetJobs"
				return hazelcast
			})
			By("checking Hazelcast CR in Running state", func() {
				evaluateReadyMembers(hzLookupKey)
			})
			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobCompleted)
		})

		It("persists JetJob on a new cluster when LosslessRestartEnabled", Tag(Fast|EE|AnyCloud), func() {
			longRunJar := "jet-pipeline-longrun-2.0.0.jar"
			setLabelAndCRName("jj-5")

			hazelcast := hazelcastconfig.JetWithLosslessRestart(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/jetJobs", labels)
			CreateHazelcastCR(hazelcast)

			By("creating JetJob CR")
			jj := hazelcastconfig.JetJob(longRunJar, hzLookupKey.Name, jjLookupKey, labels)
			t := Now()
			Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())

			checkJetJobStatus(jjLookupKey, hazelcastcomv1alpha1.JetJobRunning)

			By("Checking the JetJob jar is running")
			logs := InitLogs(t, hzLookupKey)
			logReader := test.NewLogReader(logs)
			defer logReader.Close()
			test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
				Should(ContainElements(
					MatchRegexp(fmt.Sprintf(".*\\[%s\\/\\w+#\\d+\\]\\s+SimpleEvent\\(timestamp=.*,\\s+sequence=\\d+\\).*", jj.Name))))

			By("creating HotBackup CR")
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
			assertHotBackupSuccess(hotBackup, 20*Minute)

			By("deleting Hazelcast CR")
			RemoveHazelcastCR(hazelcast)

			By("creating a new Hazelcast cluster from hot-backup")
			hazelcast = hazelcastconfig.JetWithRestore(hzLookupKey, ee, hotBackup.Name, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("checking the JetJob jar is running in the new Hazelcast cluster")
			test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
				Should(ContainElements(
					MatchRegexp(fmt.Sprintf(".*\\[%s\\/\\w+#\\d+\\]\\s+SimpleEvent\\(timestamp=.*,\\s+sequence=\\d+\\).*", jj.Name))))
		})
	})
})
