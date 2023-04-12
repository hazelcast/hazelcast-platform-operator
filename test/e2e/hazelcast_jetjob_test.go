package e2e

import (
	"context"
	"fmt"
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

	It("should execute JetJob successfully", Label("fast"), func() {
		setLabelAndCRName("jj-1")

		hazelcast := hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/jetJobs", labels)
		hazelcast.Spec.ClusterSize = pointer.Int32(1)
		hazelcast.Spec.JetEngineConfiguration.ResourceUploadEnabled = true
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
	})

	It("should change JetJob status", Label("fast"), func() {
		setLabelAndCRName("jj-2")

		hazelcast := hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/jetJobs", labels)
		hazelcast.Spec.ClusterSize = pointer.Int32(1)
		hazelcast.Spec.JetEngineConfiguration.ResourceUploadEnabled = true
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

		k8sClient.Get(context.Background(), jjLookupKey, jj)
		jj.Spec.State = hazelcastv1alpha1.SuspendedJobState
		Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
		checkJetJobStatus(hazelcastv1alpha1.JetJobSuspended)

		k8sClient.Get(context.Background(), jjLookupKey, jj)
		jj.Spec.State = hazelcastv1alpha1.RunningJobState
		Expect(k8sClient.Update(context.Background(), jj)).Should(Succeed())
		checkJetJobStatus(hazelcastv1alpha1.JetJobRunning)
	})
})
