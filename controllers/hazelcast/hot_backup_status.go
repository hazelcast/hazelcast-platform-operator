package hazelcast

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
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
