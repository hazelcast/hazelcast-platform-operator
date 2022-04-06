package hazelcast

import (
	"context"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type hotBackupStatusBuilder struct {
	state hazelcastv1alpha1.HotBackupState
}

func hotBackupState(hbs HotRestartState, currentState hazelcastv1alpha1.HotBackupState) hazelcastv1alpha1.HotBackupState {
	if currentState == hazelcastv1alpha1.HotBackupFailure {
		return currentState
	}
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
		if currentState == hazelcastv1alpha1.HotBackupInProgress || currentState == hazelcastv1alpha1.HotBackupNotStarted {
			return hazelcastv1alpha1.HotBackupSuccess
		}
	default:
		return hazelcastv1alpha1.HotBackupUnknown
	}
	return currentState
}

func updateHotBackupStatus(ctx context.Context, c client.Client, h *hazelcastv1alpha1.HotBackup, options hotBackupStatusBuilder) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
