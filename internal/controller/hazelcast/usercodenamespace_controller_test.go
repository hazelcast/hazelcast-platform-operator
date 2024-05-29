package hazelcast

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func Test_UcnReconciler_mtlsClientShouldBeRecreated(t *testing.T) {
	RegisterFailHandler(fail(t))

	nn, h, ucn := defaultCrsUCN()
	hs, _ := json.Marshal(h.Spec)
	h.ObjectMeta.Annotations = map[string]string{
		n.LastSuccessfulSpecAnnotation: string(hs),
	}
	k8sClient := fakeK8sClient(h, ucn)

	tlsConfig := setupTlsConfig(k8sClient, nn.Namespace)
	defer ucnFakeMTLSHttpServer(tlsConfig)()

	fakeHzClient, _, _ := defaultFakeClientAndService()
	cr := &fakeHzClientRegistry{}
	cr.Set(nn, &fakeHzClient)

	r := NewUserCodeNamespaceReconciler(
		k8sClient,
		ctrl.Log.WithName("test").WithName("Hazelcast"),
		nil, nil,
		cr,
		mtls.NewHttpClientRegistry())
	_, err := r.Reconcile(context.TODO(), ctrl.Request{NamespacedName: nn})
	Expect(err).Should(BeNil())
	Expect(k8sClient.Get(context.TODO(), nn, ucn)).Should(Succeed())
	Expect(ucn.Status.State).Should(Equal(hazelcastv1alpha1.UserCodeNamespaceSuccess))
}

func defaultCrsUCN() (types.NamespacedName, *hazelcastv1alpha1.Hazelcast, *hazelcastv1alpha1.UserCodeNamespace) {
	nn := types.NamespacedName{
		Name:      "hazelcast",
		Namespace: "default",
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: hazelcastv1alpha1.HazelcastSpec{
			UserCodeNamespaces: &hazelcastv1alpha1.UserCodeNamespacesConfig{},
		},
		Status: hazelcastv1alpha1.HazelcastStatus{
			Phase: hazelcastv1alpha1.Running,
		},
	}
	hs, _ := json.Marshal(h.Spec)
	h.ObjectMeta.Annotations = map[string]string{
		n.LastSuccessfulSpecAnnotation: string(hs),
	}
	ucn := &hazelcastv1alpha1.UserCodeNamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: hazelcastv1alpha1.UserCodeNamespaceSpec{
			HazelcastResourceName: h.Name,
			BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
				SecretName: "my-secret",
				BucketURI:  "s3://my-bucket-uri",
			},
		},
	}
	controllerutil.AddFinalizer(ucn, n.Finalizer)
	ucns, _ := json.Marshal(h.Spec)
	ucn.ObjectMeta.Annotations = map[string]string{
		n.LastAppliedSpecAnnotation: string(ucns),
	}
	return nn, h, ucn
}

func ucnFakeMTLSHttpServer(tlsConfig *tls.Config) func() {

	ts, err := fakeMtlsHttpServer(fmt.Sprintf("%s:%d", defaultMemberIP, hzclient.AgentPort),
		tlsConfig,
		func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(200)
			_, _ = writer.Write([]byte{})
		})
	Expect(err).Should(BeNil())
	return ts.Close
}
