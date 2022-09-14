package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	. "github.com/hazelcast/hazelcast-platform-operator/test"
	//+kubebuilder:scaffold:imports
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
)

var controllerManagerName = types.NamespacedName{
	Name: GetControllerManagerName(),
	// Namespace is set in init() function
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	SpecLabelsChecker()
	RunSpecs(t, GetSuiteName())
}

var _ = SynchronizedBeforeSuite(func() []byte {
	cfg := setupEnv()

	if ee {
		err := platform.FindAndSetPlatform(cfg)
		Expect(err).NotTo(HaveOccurred())
		if platform.GetType() == platform.OpenShift {
			cleanUpHostPath("default", "/tmp", "hazelcast")
		}
	}

	return []byte{}
}, func(bytes []byte) {
	cfg := setupEnv()
	err := platform.FindAndSetPlatform(cfg)
	Expect(err).NotTo(HaveOccurred())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	if ee {
		if platform.GetType() == platform.OpenShift {
			cleanUpHostPath("default", "/tmp", "hazelcast")
		}
	}
})

func setupEnv() *rest.Config {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = hazelcastcomv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	controllerManagerName.Namespace = hzNamespace
	setCRNamespace(hzNamespace)

	return cfg
}
