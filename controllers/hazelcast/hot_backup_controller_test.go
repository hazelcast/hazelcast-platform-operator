package hazelcast

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func TestHotBackupReconciler_shouldSetStatusToFailedIfHazelcastCRNotFound(t *testing.T) {
	RegisterFailHandler(fail(t))
	n := types.NamespacedName{
		Name:      "hazelcast",
		Namespace: "default",
	}
	hb := &hazelcastv1alpha1.HotBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
		},
		Spec: hazelcastv1alpha1.HotBackupSpec{
			HazelcastResourceName: "hazelcast",
		},
	}

	r := hotBackupReconcilerWithCRs(hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err == nil {
		t.Errorf("Error executing Reconcile to return error but returned nil")
	}

	_ = r.Client.Get(context.TODO(), n, hb)
	Expect(hb.Status.State).Should(Equal(hazelcastv1alpha1.HotBackupFailure))
	Expect(hb.Status.Message).Should(Not(BeEmpty()))
}

func fail(t *testing.T) func(message string, callerSkip ...int) {
	return func(message string, callerSkip ...int) {
		t.Errorf(message)
	}
}

func hotBackupReconcilerWithCRs(initObjs ...client.Object) HotBackupReconciler {
	return HotBackupReconciler{
		Client:    fakeClient(initObjs...),
		Log:       ctrl.Log.WithName("test").WithName("Hazelcast"),
		cancelMap: make(map[types.NamespacedName]context.CancelFunc),
		backup:    make(map[types.NamespacedName]struct{}),
	}
}
