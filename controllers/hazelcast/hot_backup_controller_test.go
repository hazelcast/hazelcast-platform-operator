package hazelcast

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/robfig/cron/v3"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/onsi/gomega"

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
			Name:        n.Name,
			Namespace:   n.Namespace,
			Annotations: make(map[string]string),
		},
		Spec: hazelcastv1alpha1.HotBackupSpec{
			HazelcastResourceName: "hazelcast",
			Schedule:              "0 23 31 2 *",
		},
	}
	r := hotBackupReconcilerWithCRs(h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err != nil {
		t.Errorf("Error executing Reconcile: %v", err)
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
		t.Errorf("Error executing Reconcile: %v", err)
	}

	Expect(r.cron.Entries()).Should(BeEmpty())
	r.scheduled.Range(func(key, value interface{}) bool {
		t.Errorf("Scheduled map should be empty. But contains key: %v value: %v", key, value)
		return false
	})
}

func TestHotBackupReconciler_shouldSetStatusToFailedWhenHbCallFails(t *testing.T) {
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
			HazelcastResourceName: n.Name,
		},
	}

	ts, err := fakeHttpServer(hazelcastUrl(h), func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI == hotBackup {
			writer.WriteHeader(500)
			_, _ = writer.Write([]byte("{\"status\":\"failed\"}"))
		} else {
			writer.WriteHeader(200)
			_, _ = writer.Write([]byte("{\"status\":\"success\"}"))
		}
	})
	if err != nil {
		t.Errorf("Failed to start fake HTTP server: %v", err)
	}
	defer ts.Close()

	r := hotBackupReconcilerWithCRs(h, hb)
	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	Expect(err).Should(BeNil())

	_ = r.Client.Get(context.TODO(), n, hb)
	Expect(hb.Status.State).Should(Equal(hazelcastv1alpha1.HotBackupPending))

	Eventually(func() hazelcastv1alpha1.HotBackupState {
		_ = r.Client.Get(context.TODO(), n, hb)
		return hb.Status.State
	}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.HotBackupFailure))
}

func TestHotBackupReconciler_shouldNotTriggerHotBackupTwice(t *testing.T) {
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
			HazelcastResourceName: n.Name,
		},
	}

	var restCallWg sync.WaitGroup
	restCallWg.Add(1)
	var hotBackupTriggers int32
	ts, err := fakeHttpServer(hazelcastUrl(h), func(writer http.ResponseWriter, request *http.Request) {
		if request.RequestURI == hotBackup {
			t.Log(request.Method, request.URL)
			atomic.AddInt32(&hotBackupTriggers, 1)
			restCallWg.Wait()
		}
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("{\"status\":\"success\"}"))
	})
	if err != nil {
		t.Errorf("Failed to start fake HTTP server: %v", err)
	}
	defer ts.Close()

	r := hotBackupReconcilerWithCRs(h, hb)

	var reconcileWg sync.WaitGroup
	reconcileWg.Add(1)
	go func() {
		defer reconcileWg.Done()
		_, _ = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	}()

	Eventually(func() hazelcastv1alpha1.HotBackupState {
		_ = r.Client.Get(context.TODO(), n, hb)
		return hb.Status.State
	}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.HotBackupInProgress))

	reconcileWg.Add(1)
	go func() {
		defer reconcileWg.Done()
		_, _ = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	}()
	restCallWg.Done()
	reconcileWg.Wait()

	Expect(hotBackupTriggers).Should(Equal(int32(1)))
}

func TestHotBackupReconciler_shouldUpdateWhenScheduledBackupChangedToInstantBackup(t *testing.T) {
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
		Status: hazelcastv1alpha1.HazelcastStatus{Phase: hazelcastv1alpha1.Running},
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
	ts, err := fakeHttpServer(hazelcastUrl(h), func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("{\"status\":\"success\"}"))
	})
	if err != nil {
		t.Errorf("Failed to start fake HTTP server: %v", err)
	}
	defer ts.Close()

	r := hotBackupReconcilerWithCRs(h, hb)
	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err != nil {
		t.Errorf("Error executing Reconcile: %v", err)
	}

	Expect(r.cron.Entries()).Should(HaveLen(1))

	Expect(r.Client.Get(context.TODO(), n, hb)).Should(Succeed())
	hb.Spec.Schedule = ""
	Expect(r.Client.Update(context.TODO(), hb)).Should(Succeed())

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err != nil {
		t.Errorf("Error executing Reconcile: %v", err)
	}
	Expect(r.cron.Entries()).Should(BeEmpty())
	r.scheduled.Range(func(key, value interface{}) bool {
		t.Errorf("Scheduled map should be empty. But contains key: %v value: %v", key, value)
		return false
	})
}

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
			Schedule:              "0 23 31 2 *",
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
		Client: fakeClient(initObjs...),
		Log:    ctrl.Log.WithName("test").WithName("Hazelcast"),
		cron:   cron.New(),
	}
}
