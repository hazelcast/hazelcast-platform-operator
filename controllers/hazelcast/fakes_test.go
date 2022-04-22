package hazelcast

import (
	"net"
	"net/http"
	"net/http/httptest"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func fakeClient(initObjs ...client.Object) client.Client {
	scheme, _ := hazelcastv1alpha1.SchemeBuilder.
		Register(&hazelcastv1alpha1.Hazelcast{}, &hazelcastv1alpha1.HazelcastList{}, &v1.ClusterRole{}, &v1.ClusterRoleBinding{}).
		Build()
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()
}

func fakeHttpServer(url string, handler http.HandlerFunc) (*httptest.Server, error) {
	l, err := net.Listen("tcp", url)
	if err != nil {
		return nil, err
	}
	ts := httptest.NewUnstartedServer(handler)
	_ = ts.Listener.Close()
	ts.Listener = l
	ts.Start()
	return ts, nil
}
