package e2e

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Backup", Label("backup"), func() {
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1alpha1.HotBackup{}, &hazelcastv1alpha1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Map{}, &hazelcastv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should fail if bucket credential of external backup in secret is not correct", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hb-1")
		ctx := context.Background()
		clusterSize := int32(1)
		var pvcSizeInMb = 1
		var bucketURI = "gs://operator-e2e-external-backup"
		var secretName = "br-incorrect-secret-gcp"
		var credential = `{
  "type": "service_account",
  "project_id": "project",
  "private_key_id": "12345678910111213",
  "private_key": PRIVATE KEY",
  "client_email": "sa@project.iam.gserviceaccount.com",
  "client_id": "123456789",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/sa%40project.iam.gserviceaccount.com"
}`

		By("creating cluster with external backup enabled")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("create bucket credential secret")
		secret := corev1.Secret{}
		secret.StringData = map[string]string{
			"google-credentials-path": credential,
		}
		secret.Name = secretName
		secret.Namespace = hazelcast.Namespace
		Eventually(func() error {
			err := k8sClient.Create(ctx, &secret)
			if errors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}, 20*Second, interval).Should(Succeed())
		assertExists(types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}, &secret)

		By("triggering the backup")
		hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		Eventually(func() hazelcastv1alpha1.HotBackupState {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hotBackup.Namespace, Name: hotBackup.Name}, hotBackup)
			Expect(err).ToNot(HaveOccurred())
			return hotBackup.Status.State
		}, 20*Second, interval).Should(Equal(hazelcastv1alpha1.HotBackupFailure))
	})
})
