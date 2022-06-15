package hazelcast

import (
	"context"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type hotBackupOptionsBuilder struct {
	status  hazelcastv1alpha1.HotBackupState
	err     error
	message string
}

func hbWithStatus(s hazelcastv1alpha1.HotBackupState) hotBackupOptionsBuilder {
	return hotBackupOptionsBuilder{
		status: s,
	}
}

func failedHbStatus(err error) hotBackupOptionsBuilder {
	return hotBackupOptionsBuilder{
		status:  hazelcastv1alpha1.HotBackupFailure,
		err:     err,
		message: err.Error(),
	}
}

func pendingHbStatus() hotBackupOptionsBuilder {
	return hotBackupOptionsBuilder{
		status: hazelcastv1alpha1.HotBackupPending,
	}
}

func updateHotBackupStatus(ctx context.Context, c client.Client, hb *hazelcastv1alpha1.HotBackup, options hotBackupOptionsBuilder) (ctrl.Result, error) {
	hb.Status.State = options.status
	hb.Status.Message = options.message
	err := c.Status().Update(ctx, hb)
	if options.status == hazelcastv1alpha1.HotBackupFailure {
		return ctrl.Result{}, options.err
	}
	return ctrl.Result{}, err
}

func hotBackupState(hbs HotRestartState, currentState hazelcastv1alpha1.HotBackupState, backupType hazelcastv1alpha1.BackupType) hazelcastv1alpha1.HotBackupState {
	switch hbs.BackupTaskState {
	case "NOT_STARTED":
		if currentState == hazelcastv1alpha1.HotBackupUnknown {
			return hazelcastv1alpha1.HotBackupNotStarted
		}
	case "IN_PROGRESS":
		return hazelcastv1alpha1.HotBackupInProgress
	case "FAILURE":
		return hazelcastv1alpha1.HotBackupFailure
	case "SUCCESS":
		if currentState == hazelcastv1alpha1.HotBackupUnknown {
			switch backupType {
			case hazelcastv1alpha1.External:
				return hazelcastv1alpha1.HotBackupWaiting

			default:
				return hazelcastv1alpha1.HotBackupSuccess
			}
		}
	default:
		return hazelcastv1alpha1.HotBackupUnknown
	}
	return currentState
}
