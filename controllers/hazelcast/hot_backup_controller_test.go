package hazelcast

import (
	"context"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func TestHotBackupReconciler_shouldScheduleHotBackupExecution(t *testing.T) {
	RegisterFailHandler(fail(t))
	n := types.NamespacedName{
		Name:      "hazelcast",
		Namespace: "default",
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
		},
		Status: hazelcastv1alpha1.HazelcastStatus{
			Phase: hazelcastv1alpha1.Running,
		},
	}
	hb := &hazelcastv1alpha1.HotBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
		},
		Spec: hazelcastv1alpha1.HotBackupSpec{
			HazelcastResourceName: "hazelcast",
			Schedule:              "0 23 31 2 *",
		},
	}
	r := hotBackupReconcilerWithCRs(h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err != nil {
		t.Errorf("Error executing Reconcile: %e", err)
	}
	load, _ := r.scheduled.Load(n)
	Expect(load).ShouldNot(BeNil())
	Expect(r.cron.Entries()).Should(HaveLen(1))
	Expect(r.cron.Entries()).Should(ConsistOf(
		WithTransform(func(entry cron.Entry) cron.EntryID {
			return entry.ID
		}, Equal(load.(cron.EntryID))),
	))
}

func TestHotBackupReconciler_shouldRemoveScheduledBackup(t *testing.T) {
	RegisterFailHandler(fail(t))
	n := types.NamespacedName{
		Name:      "hazelcast",
		Namespace: "default",
	}
	h := &hazelcastv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Name,
			Namespace: n.Namespace,
		},
	}
	hb := &hazelcastv1alpha1.HotBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              n.Name,
			Namespace:         n.Namespace,
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
		},
		Spec: hazelcastv1alpha1.HotBackupSpec{
			HazelcastResourceName: "hazelcast",
			Schedule:              "0 23 31 2 *",
		},
	}

	r := hotBackupReconcilerWithCRs(h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err != nil {
		t.Errorf("Error executing Reconcile: %e", err)
	}

	Expect(r.cron.Entries()).Should(BeEmpty())
	r.scheduled.Range(func(key, value interface{}) bool {
		t.Errorf("Scheduled map should be empty. But contains key: %v value: %v", key, value)
		return false
	})
}

func fail(t *testing.T) func(message string, callerSkip ...int) {
	return func(message string, callerSkip ...int) {
		t.Errorf(message)
	}
}

func hotBackupReconcilerWithCRs(h *hazelcastv1alpha1.Hazelcast, hb *hazelcastv1alpha1.HotBackup) *HotBackupReconciler {
	return NewHotBackupReconciler(fakeClient(h, hb), ctrl.Log.WithName("test").WithName("Hazelcast"))
}
