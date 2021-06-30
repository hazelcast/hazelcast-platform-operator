package controllers

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

//buildReconcileWithFakeClientWithMocks return reconcile with fake client, schemes and mock objects
func buildReconcileWithFakeClientWithMocks(objs []runtime.Object) *HazelcastReconciler {
	s := scheme.Scheme
	s.AddKnownTypes(hazelcastv1alpha1.GroupVersion, &hazelcastv1alpha1.Hazelcast{})

	r := fake.NewClientBuilder().
		WithScheme(s).
		WithRuntimeObjects(objs...).
		Build()

	return &HazelcastReconciler{
		Client: r,
		Log:    logf.Log.WithName("hazelcast"),
		Scheme: s,
	}
}
