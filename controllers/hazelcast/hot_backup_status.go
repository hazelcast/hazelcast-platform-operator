package hazelcast

import (
	"context"
	"errors"
	"time"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
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

var (
	errBackupFailed            = errors.New("Backup failed")
	errBackupMemberStateFailed = errors.New("Backup member state update failed")
	errBackupNoTask            = errors.New("Backup member state indicates no task started")
)

func waitUntilBackupSucceed(ctx context.Context, client *Client, uuid hztypes.UUID) error {
	var n int
	for {
		state := client.getTimedMemberState(ctx, uuid)
		if state == nil {
			return errBackupMemberStateFailed
		}

		s := state.TimedMemberState.MemberState.HotRestartState.BackupTaskState
		switch s {
		case "FAILURE":
			return errBackupFailed
		case "SUCCESS":
			return nil
		case "NO_TASK":
			// sometimes it takes few seconds for task to start
			if n > 10 {
				return errBackupNoTask
			}
			n++
		case "IN_PROGRESS":
			// expected, check status again (no return)
		default:
			return errors.New("Backup unknown status: " + s)

		}

		// wait for timer or context to cancel
		select {
		case <-time.After(1 * time.Second):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
