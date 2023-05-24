package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast JetJob", Label("JetJob"), func() {
	//localPort := strconv.Itoa(9000 + GinkgoParallelProcess())
	fastRunJar := "jet-pipeline-1.0.2.jar"
	longRunJar := "jet-pipeline-longrun-2.0.0.jar"

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		version := os.Getenv("HZ_VERSION")
		if strings.Contains(version, "5.1.6") || strings.Contains(version, "5.0.5") {
			Skip("We don't support Jet for 5.1.6 or 5.0.5 version")
		}
		if runningLocally() {
			return
		}
	})

	checkJetJobStatus := func(phase hazelcastv1alpha1.JetJobStatusPhase) {
		jjCheck := &hazelcastv1alpha1.JetJob{}
		Eventually(func() hazelcastv1alpha1.JetJobStatusPhase {
			err := k8sClient.Get(
				context.Background(), types.NamespacedName{Name: jjLookupKey.Name, Namespace: hzNamespace}, jjCheck)
			Expect(err).ToNot(HaveOccurred())
			return jjCheck.Status.Phase
		}, 5*Minute, interval).Should(Equal(phase))
	}

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1alpha1.JetJob{}, &hazelcastv1alpha1.JetJobList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&corev1.Secret{}, &corev1.SecretList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	DescribeTable("should execute JetJob successfully", func(secretName, url string) {
		setLabelAndCRName("jj-1")

		var hazelcast *hazelcastv1alpha1.Hazelcast
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
		checkJetJobStatus(hazelcastv1alpha1.JetJobCompleted)

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
		Entry("using jar from bucket", Label("fast"), "br-secret-gcp", "gs://operator-user-code/jetJobs"),
		Entry("using jar from remote url", Label("fast"), "", "https://storage.googleapis.com/operator-user-code-urls-public/jet-pipeline-1.0.2.jar"),
	)

	It("should change JetJob status", Label("fast"), func() {
		setLabelAndCRName("jj-2")

		hazelcast := hazelcastconfig.JetWithBucketConfigured(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/jetJobs", labels)
		hazelcast.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCR(hazelcast)

		By("creating JetJob CR")
		jj := hazelcastconfig.JetJob(longRunJar, hzLookupKey.Name, jjLookupKey, labels)
		t := Now()
		Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
		checkJetJobStatus(hazelcastv1alpha1.JetJobRunning)

		By("Checking the JetJob jar is running")
		logs := InitLogs(t, hzLookupKey)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
			Should(ContainElements(
				MatchRegexp(fmt.Sprintf(".*\\[%s\\/\\w+#\\d+\\]\\s+SimpleEvent\\(timestamp=.*,\\s+sequence=\\d+\\).*", jj.Name))))

		Expect(k8sClient.Get(context.Background(), jjLookupKey, jj)).Should(Succeed())
		jj.Spec.State = hazelcastv1alpha1.SuspendedJobState
		Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
		checkJetJobStatus(hazelcastv1alpha1.JetJobSuspended)

		Expect(k8sClient.Get(context.Background(), jjLookupKey, jj)).Should(Succeed())
		jj.Spec.State = hazelcastv1alpha1.RunningJobState
		Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
		checkJetJobStatus(hazelcastv1alpha1.JetJobRunning)
	})

	DescribeTable("should download JAR and execute JetJob", func(secretName, url string) {
		setLabelAndCRName("jj-3")

		hazelcast := hazelcastconfig.JetConfigured(hzLookupKey, ee, labels)
		hazelcast.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCR(hazelcast)

		By("creating JetJob CR")
		jj := hazelcastconfig.JetJob(fastRunJar, hzLookupKey.Name, jjLookupKey, labels)
		if secretName != "" {
			jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastv1alpha1.BucketConfiguration{
				SecretName: secretName,
				BucketURI:  url,
			}
		} else {
			jj.Spec.JetRemoteFileConfiguration.RemoteURL = url
		}
		jj.Spec.JetRemoteFileConfiguration.BucketConfiguration = &hazelcastv1alpha1.BucketConfiguration{
			SecretName: "br-secret-gcp",
			BucketURI:  "gs://operator-user-code/jetJobs",
		}

		t := Now()
		Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())
		checkJetJobStatus(hazelcastv1alpha1.JetJobCompleted)

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
		Entry("using jar from bucket", Label("fast"), "br-secret-gcp", "gs://operator-user-code/jetJobs"),
		Entry("using jar from remote url", Label("fast"), "", "https://storage.googleapis.com/operator-user-code-urls-public/jet-pipeline-1.0.2.jar"),
	)

	It("should persist jobs when lossless restart is enabled", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}

		setLabelAndCRName("jj-4")

		hazelcast := hazelcastconfig.JetWithLosslessRestart(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/jetJobs", labels)
		CreateHazelcastCR(hazelcast)

		By("creating JetJob CR")
		jj := hazelcastconfig.JetJob(longRunJar, hzLookupKey.Name, jjLookupKey, labels)
		t := Now()
		Expect(k8sClient.Create(context.Background(), jj)).Should(Succeed())

		checkJetJobStatus(hazelcastv1alpha1.JetJobRunning)

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

		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from the existing backup")
		hazelcast = hazelcastconfig.JetWithRestore(hzLookupKey, ee, hotBackup.Name, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		checkJetJobStatus(hazelcastv1alpha1.JetJobRunning)

		By("Checking the JetJob jar is running in new Hazelcast cluster")
		test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
			Should(ContainElements(
				MatchRegexp(fmt.Sprintf(".*\\[%s\\/\\w+#\\d+\\]\\s+SimpleEvent\\(timestamp=.*,\\s+sequence=\\d+\\).*", jj.Name))))
	})
})
