package integration

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/managementcenter"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"

	. "github.com/hazelcast/hazelcast-platform-operator/test"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var ctx context.Context
var cancel context.CancelFunc

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	SpecLabelsChecker()
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths:                    []string{filepath.Join("..", "..", "config", "webhook")},
			IgnoreErrorIfPathMissing: false,
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = platform.FindAndSetPlatform(cfg)
	Expect(err).NotTo(HaveOccurred())

	err = hazelcastcomv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	webhookInstallOptions := testEnv.WebhookInstallOptions
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Host:    webhookInstallOptions.LocalServingHost,
		Port:    webhookInstallOptions.LocalServingPort,
		CertDir: webhookInstallOptions.LocalServingCertDir,
	})
	Expect(err).ToNot(HaveOccurred())

	cs := &hzclient.HazelcastClientRegistry{}
	ssm := &hzclient.HzStatusServiceRegistry{}

	err = hazelcast.NewHazelcastReconciler(
		k8sManager.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Hazelcast"),
		k8sManager.GetScheme(),
		nil,
		cs,
		ssm,
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = managementcenter.NewManagementCenterReconciler(
		k8sManager.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Management Center"),
		k8sManager.GetScheme(),
		nil,
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = hazelcast.NewMapReconciler(
		k8sManager.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Map"),
		k8sManager.GetScheme(),
		nil,
		cs,
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = hazelcast.NewHotBackupReconciler(
		k8sManager.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Hot Backup"),
		nil,
		nil,
		cs,
		ssm,
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = hazelcast.NewCronHotBackupReconciler(
		k8sManager.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Cron Hot Backup"),
		k8sManager.GetScheme(),
		nil,
	).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&hazelcastcomv1alpha1.Hazelcast{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.Map{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.Cache{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.ReplicatedMap{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.Topic{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.Queue{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.CronHotBackup{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.HotBackup{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.MultiMap{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.WanReplication{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	err = (&hazelcastcomv1alpha1.ManagementCenter{}).SetupWebhookWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
