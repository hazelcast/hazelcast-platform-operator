package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	chaosmeshv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	ginkgoTypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//+kubebuilder:scaffold:imports

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
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
	if len(suiteConfig.LabelFilter) > 0 {
		if ee {
			suiteConfig.LabelFilter += "&& " + tagNames[EE]
		} else {
			suiteConfig.LabelFilter += "&& " + tagNames[OS]
		}
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

	k8sClient = NewManifestRecorder(k8sClient)
	k8sClient = NewResourceTracker(k8sClient)

	controllerManagerName.Namespace = hzNamespace
	setCRNamespace(hzNamespace)

	return cfg
}

var recordedManifests = make(map[types.NamespacedName]io.Writer)

// manifestRecorder keeps track of applied manifests
type manifestRecorder struct {
	client.Client
}

func NewManifestRecorder(client client.Client) *manifestRecorder {
	return &manifestRecorder{Client: client}
}

func (d *manifestRecorder) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	gvk, _, err := d.Client.Scheme().ObjectKinds(obj)
	if err != nil {
		return err
	}

	if len(gvk) == 0 {
		// skip unknown objects
		return d.Client.Create(ctx, obj, opts...)
	}

	serializer := json.NewYAMLSerializer(
		json.DefaultMetaFactory, nil, nil,
	)

	sink, ok := recordedManifests[hzLookupKey]
	if !ok {
		sink = new(bytes.Buffer)
	}
	fmt.Fprintf(sink, "---\n")
	fmt.Fprintf(sink, "apiVersion: %s/%s\n", gvk[0].Group, gvk[0].Version)
	fmt.Fprintf(sink, "kind: %s\n", gvk[0].Kind)
	if err := serializer.Encode(obj, sink); err != nil {
		return err
	}
	recordedManifests[hzLookupKey] = sink

	return d.Client.Create(ctx, obj, opts...)
}

// resourceTracker keeps track of created resources
type resourceTracker struct {
	client.Client

	resources []client.Object
}

func NewResourceTracker(client client.Client) *resourceTracker {
	return &resourceTracker{Client: client}
}

func (t *resourceTracker) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	t.resources = append(t.resources, obj)
	return t.Client.Create(ctx, obj, opts...)
}

func (t *resourceTracker) Cleanup(ctx context.Context) {
	// trackedResources is a stack, we want to remove resources in reverse order
	for i := len(t.resources) - 1; i >= 0; i-- {
		err := t.Client.Delete(ctx, t.resources[i])
		if err != nil {
			GinkgoWriter.Printf("resourceTracker(%v) error: %v\n", t.resources[i].GetName(), err)
		}
	}
	t.resources = nil
}

func Cleanup(ctx context.Context) {
	if c, ok := k8sClient.(interface {
		Cleanup(ctx context.Context)
	}); ok {
		c.Cleanup(ctx)
	}
}
