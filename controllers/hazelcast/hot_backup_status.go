package hazelcast

import (
	"context"
	"time"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
)

type HotBackupStatusApplier interface {
	HotBackupStatusApply(ms *hazelcastv1beta1.HotBackupStatus)
}

type withHotBackupState hazelcastv1beta1.HotBackupState

func (w withHotBackupState) HotBackupStatusApply(ms *hazelcastv1beta1.HotBackupStatus) {
	ms.State = hazelcastv1beta1.HotBackupState(w)
	if hazelcastv1beta1.HotBackupState(w) == hazelcastv1beta1.HotBackupSuccess {
		ms.Message = ""
	}
}

type withHotBackupFailedState string

func (w withHotBackupFailedState) HotBackupStatusApply(ms *hazelcastv1beta1.HotBackupStatus) {
	ms.State = hazelcastv1beta1.HotBackupFailure
	ms.Message = string(w)
}

type withHotBackupMessage string

func (w withHotBackupMessage) HotBackupStatusApply(ms *hazelcastv1beta1.HotBackupStatus) {
	ms.Message = string(w)
}

type withHotBackupBackupUUIDs []string

func (w withHotBackupBackupUUIDs) HotBackupStatusApply(ms *hazelcastv1beta1.HotBackupStatus) {
	ms.BackupUUIDs = w
}

func (r *HotBackupReconciler) updateStatus(ctx context.Context, name types.NamespacedName, recOption controllers.ReconcilerOption, options ...HotBackupStatusApplier) (ctrl.Result, error) {
	hb := &hazelcastv1beta1.HotBackup{}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		if err := r.Get(ctx, name, hb); err != nil {
			return err
		}
		for _, applier := range options {
			applier.HotBackupStatusApply(&hb.Status)
		}
		return r.Status().Update(ctx, hb)
	})

	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if hb.Status.State == hazelcastv1beta1.HotBackupPending {
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, err
}
