package hazelcast

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func hotBackupState(hbs HotRestartState, currentState hazelcastv1alpha1.HotBackupState) hazelcastv1alpha1.HotBackupState {
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
			return hazelcastv1alpha1.HotBackupSuccess
		}
	default:
		return hazelcastv1alpha1.HotBackupUnknown
	}
	return currentState
}
