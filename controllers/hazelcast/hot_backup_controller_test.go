package hazelcast

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	clientTypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/matchers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

var (
	defaultMemberIP = "127.0.0.1"
)

func TestHotBackupReconciler_shouldBeSuccessful(t *testing.T) {
	RegisterFailHandler(fail(t))
	nn, h, hb := defaultCRs()
	fakeHzClient, fakeHzStatusService, _ := defaultFakeClientAndService()

	defer defaultFakeHttpServer()()

	cr := &fakeHzClientRegistry{}
	sr := &fakeHzStatusServiceRegistry{}
	cr.Set(nn, &fakeHzClient)
	sr.Set(nn, &fakeHzStatusService)

	r := hotBackupReconcilerWithCRs(cr, sr, h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	Expect(err).Should(BeNil())

	Eventually(func() hazelcastv1alpha1.HotBackupState {
		_ = r.Client.Get(context.TODO(), nn, hb)
		return hb.Status.State
	}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.HotBackupSuccess))
}

func TestHotBackupReconciler_shouldSetStatusToFailedWhenHbCallFails(t *testing.T) {
	RegisterFailHandler(fail(t))
	nn, h, hb := defaultCRs()

	fakeHzClient, fakeHzStatusService, _ := defaultFakeClientAndService()
	fakeHzClient.tInvokeOnRandomTarget = func(ctx context.Context, req *hazelcast.ClientMessage, opts *hazelcast.InvokeOptions) (*hazelcast.ClientMessage, error) {
		if req.Type() == codec.MCTriggerHotRestartBackupCodecRequestMessageType {
			return nil, fmt.Errorf("Backup trigger request failed")
		}
		return nil, nil
	}

	defer defaultFakeHttpServer()()

	sr := &fakeHzStatusServiceRegistry{}
	cr := &fakeHzClientRegistry{}

	sr.Set(nn, &fakeHzStatusService)
	cr.Set(nn, &fakeHzClient)

	r := hotBackupReconcilerWithCRs(cr, sr, h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	Expect(err).Should(BeNil())

	Eventually(func() hazelcastv1alpha1.HotBackupState {
		_ = r.Client.Get(context.TODO(), nn, hb)
		return hb.Status.State
	}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.HotBackupFailure))
}

func TestHotBackupReconciler_shouldSetStatusToFailedWhenTimedMemberStateFails(t *testing.T) {
	RegisterFailHandler(fail(t))
	nn, h, hb := defaultCRs()

	fakeHzClient, fakeHzStatusService, mm := defaultFakeClientAndService()
	fakeHzStatusService.timedMemberStateMap[mm[0].UUID].TimedMemberState.MemberState.HotRestartState.BackupTaskState = "FAILURE"

	defer defaultFakeHttpServer()()

	sr := &fakeHzStatusServiceRegistry{}
	cr := &fakeHzClientRegistry{}

	sr.Set(nn, &fakeHzStatusService)
	cr.Set(nn, &fakeHzClient)

	r := hotBackupReconcilerWithCRs(cr, sr, h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	Expect(err).Should(BeNil())

	Eventually(func() hazelcastv1alpha1.HotBackupState {
		_ = r.Client.Get(context.TODO(), nn, hb)
		return hb.Status.State
	}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.HotBackupFailure))
}

func TestHotBackupReconciler_shouldNotTriggerHotBackupTwice(t *testing.T) {
	RegisterFailHandler(fail(t))
	nn, h, hb := defaultCRs()

	var restCallWg sync.WaitGroup
	restCallWg.Add(1)
	var hotBackupTriggers int32

	fakeHzClient, fakeHzStatusService, _ := defaultFakeClientAndService()
	fakeHzClient.tInvokeOnRandomTarget = func(ctx context.Context, req *hazelcast.ClientMessage, opts *hazelcast.InvokeOptions) (*hazelcast.ClientMessage, error) {
		if req.Type() == codec.MCTriggerHotRestartBackupCodecRequestMessageType {
			atomic.AddInt32(&hotBackupTriggers, 1)
			restCallWg.Wait()
		}
		return nil, nil
	}

	defer defaultFakeHttpServer()()

	sr := &fakeHzStatusServiceRegistry{}
	cr := &fakeHzClientRegistry{}
	sr.Set(nn, &fakeHzStatusService)
	cr.Set(nn, &fakeHzClient)

	r := hotBackupReconcilerWithCRs(cr, sr, h, hb)
	var reconcileWg sync.WaitGroup
	reconcileWg.Add(1)
	go func() {
		defer reconcileWg.Done()
		_, _ = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	}()

	Eventually(func() hazelcastv1alpha1.HotBackupState {
		_ = r.Client.Get(context.TODO(), nn, hb)
		return hb.Status.State
	}, 2*time.Second, 100*time.Millisecond).Should(Equal(hazelcastv1alpha1.HotBackupInProgress))

	reconcileWg.Add(1)
	go func() {
		defer reconcileWg.Done()
		_, _ = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	}()
	restCallWg.Done()
	reconcileWg.Wait()

	Expect(hotBackupTriggers).Should(Equal(int32(1)))
}

func TestHotBackupReconciler_shouldCancelContextIfHotbackupCRIsDeleted(t *testing.T) {
	RegisterFailHandler(fail(t))
	nn, h, hb := defaultCRs()

	fakeHzClient, fakeHzStatusService, _ := defaultFakeClientAndService()
	defer defaultFakeHttpServer()()

	cr := &fakeHzClientRegistry{}
	sr := &fakeHzStatusServiceRegistry{}
	cr.Set(nn, &fakeHzClient)
	sr.Set(nn, &fakeHzStatusService)

	r := hotBackupReconcilerWithCRs(cr, sr, h, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	Expect(err).Should(BeNil())

	Expect(r.Client.Get(context.TODO(), nn, hb)).Should(Succeed())
	Expect(r.cancelMap).Should(&matchers.HaveLenMatcher{Count: 1})

	timeNow := metav1.Now()
	hb.ObjectMeta.DeletionTimestamp = &timeNow
	err = r.Client.Update(context.TODO(), hb)
	Expect(err).Should(BeNil())

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	Expect(err).Should(BeNil())

	Expect(r.cancelMap).Should(&matchers.HaveLenMatcher{Count: 0})
}

func TestHotBackupReconciler_shouldNotSetStatusToFailedIfHazelcastCRNotFound(t *testing.T) {
	RegisterFailHandler(Fail)
	nn, _, hb := defaultCRs()

	r := hotBackupReconcilerWithCRs(&fakeHzClientRegistry{}, &fakeHzStatusServiceRegistry{}, hb)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Errorf("Error expecting Reconcile to return without error")
	}

	_ = r.Client.Get(context.TODO(), nn, hb)
	Expect(hb.Status.State).ShouldNot(Equal(hazelcastv1alpha1.HotBackupFailure))
}

func fail(t *testing.T) func(message string, callerSkip ...int) {
	return func(message string, callerSkip ...int) {
		t.Errorf(message)
	}
}

func hotBackupReconcilerWithCRs(clientReg hzclient.ClientRegistry, serviceReg hzclient.StatusServiceRegistry, initObjs ...client.Object) *HotBackupReconciler {
	return NewHotBackupReconciler(
		fakeK8sClient(initObjs...),
		ctrl.Log.WithName("test").WithName("Hazelcast"),
		nil,
		&mtls.Client{},
		clientReg,
		serviceReg,
	)

}

func defaultFakeClientAndService() (fakeHzClient, fakeHzStatusService, []cluster.MemberInfo) {
	defaultMemberAddress := cluster.Address(defaultMemberIP + ":5701")
	mm := []cluster.MemberInfo{
		{
			Address: defaultMemberAddress,
			UUID:    clientTypes.NewUUID(),
		},
		{
			Address: defaultMemberAddress,
			UUID:    clientTypes.NewUUID(),
		},
		{
			Address: defaultMemberAddress,
			UUID:    clientTypes.NewUUID(),
		},
	}

	fakeHzClient := fakeHzClient{
		tOrderedMembers:          mm,
		tIsClientConnected:       true,
		tAreAllMembersAccessible: true,
		tRunning:                 true,
	}

	timedMemberState := &codecTypes.TimedMemberStateWrapper{}
	timedMemberState.TimedMemberState.MemberState.HotRestartState.BackupTaskState = "SUCCESS"
	fakeHzStatusService := fakeHzStatusService{
		timedMemberStateMap: map[clientTypes.UUID]*codecTypes.TimedMemberStateWrapper{
			mm[0].UUID: timedMemberState,
			mm[1].UUID: timedMemberState,
			mm[2].UUID: timedMemberState,
		},
		Status: &hzclient.Status{
			MemberDataMap: map[clientTypes.UUID]*hzclient.MemberData{
				mm[0].UUID: {
					Address: mm[0].Address.String(),
				},
				mm[1].UUID: {
					Address: mm[1].Address.String(),
				},
				mm[2].UUID: {
					Address: mm[2].Address.String(),
				},
			},
		},
	}
	return fakeHzClient, fakeHzStatusService, mm
}

func defaultCRs() (types.NamespacedName, *hazelcastv1alpha1.Hazelcast, *hazelcastv1alpha1.HotBackup) {
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
			Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
				BaseDir: "basedir",
			},
		},
		Status: hazelcastv1alpha1.HazelcastStatus{
			Phase: hazelcastv1alpha1.Running,
		},
	}
	hb := &hazelcastv1alpha1.HotBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: hazelcastv1alpha1.HotBackupSpec{
			HazelcastResourceName: nn.Name,
		},
	}
	return nn, h, hb
}

func defaultFakeHttpServer() func() {
	ts, err := fakeHttpServer(fmt.Sprintf("%s:%d", defaultMemberIP, hzclient.AgentPort), func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte(`{"backups" : ["backup-123"]}`))
	})
	Expect(err).Should(BeNil())
	return ts.Close
}
