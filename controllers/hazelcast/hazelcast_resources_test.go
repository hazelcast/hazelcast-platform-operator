package hazelcast

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
)

func Test_clientShutdownWhenConnectionNotEstablished(t *testing.T) {
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hazelcast",
			Namespace: "default",
		},
	}
	r := reconcilerWithCR(h)
	hzclient.Clients.Store(types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, &hzclient.Client{})

	err := r.executeFinalizer(context.Background(), h, ctrl.Log)
	if err != nil {
		t.Errorf("Error while executing finilazer: %v.", err)
	}
}

func reconcilerWithCR(h *hazelcastv1alpha1.Hazelcast) HazelcastReconciler {
	return HazelcastReconciler{
		Client: fakeClient(h),
	}
}
