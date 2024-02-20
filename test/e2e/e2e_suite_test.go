package e2e

import (
	"fmt"
	"testing"

	chaosmeshv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	ginkgoTypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	//+kubebuilder:scaffold:imports
)

var k8sClient client.Client

var controllerManagerName = types.NamespacedName{
	Name: GetControllerManagerName(),
	// Namespace is set in init() function
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, _ := GinkgoConfiguration()
	SetLicenseLabelFilters(&suiteConfig)

	RunSpecs(t, GetSuiteName(), suiteConfig)
}

func SetLicenseLabelFilters(suiteConfig *ginkgoTypes.SuiteConfig) {
	if ee {
		suiteConfig.LabelFilter += fmt.Sprintf(" && %s", tagNames[EE])
	} else {
		suiteConfig.LabelFilter += fmt.Sprintf(" && %s", tagNames[OS])
	}
}

var _ = SynchronizedBeforeSuite(func() []byte {
	cfg := setupEnv()

	if ee {
		err := platform.FindAndSetPlatform(cfg)
		Expect(err).NotTo(HaveOccurred())
	}

	return []byte{}
}, func(bytes []byte) {
	cfg := setupEnv()
	err := platform.FindAndSetPlatform(cfg)
	Expect(err).NotTo(HaveOccurred())

})

func setupEnv() *rest.Config {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	err := hazelcastcomv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = chaosmeshv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = routev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	controllerManagerName.Namespace = hzNamespace
	setCRNamespace(hzNamespace)

	return cfg
}
