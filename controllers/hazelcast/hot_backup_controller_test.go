package hazelcast

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/client"
	hzconfig "github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/config"
)

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

	ts, err := fakeHttpServer(hzconfig.HazelcastUrl(h), func(writer http.ResponseWriter, request *http.Request) {
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

	hzclient.Clients.Store(types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, &hzclient.Client{
		Status: &hzclient.Status{
			MemberMap: make(map[hztypes.UUID]*hzclient.MemberData),
		},
	})

	var restCallWg sync.WaitGroup
	restCallWg.Add(1)
	var hotBackupTriggers int32
	ts, err := fakeHttpServer(hzconfig.HazelcastUrl(h), func(writer http.ResponseWriter, request *http.Request) {
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

func TestHotBackupReconciler_shouldCancelContextIfHazelcastCRIsDeleted(t *testing.T) {
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
		},
	}
	ts, err := fakeHttpServer(hzconfig.HazelcastUrl(h), func(writer http.ResponseWriter, request *http.Request) {
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

	Expect(r.Client.Get(context.TODO(), n, hb)).Should(Succeed())
	Expect(r.cancelMap).Should(&matchers.HaveLenMatcher{Count: 1})

	timeNow := metav1.Now()
	hb.ObjectMeta.DeletionTimestamp = &timeNow
	err = r.Client.Update(context.TODO(), hb)
	if err != nil {
		t.Errorf("Error on update hotbackup: %v", err)
	}

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: n})
	if err != nil {
		t.Errorf("Error executing Reconcile: %v", err)
	}

	Expect(r.cancelMap).Should(&matchers.HaveLenMatcher{Count: 0})
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
