package backup

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

var backupBackoff = wait.Backoff{
	Steps:    8,
	Duration: 10 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
}

type ClusterBackup struct {
	statusService hzclient.StatusService
	backupService hzclient.BackupService
	members       map[hztypes.UUID]*hzclient.MemberData
	cancelOnce    sync.Once
}

var (
	errBackupClientNoMembers = errors.New("client couldnt connect to members")
)

func NewClusterBackup(ss hzclient.StatusService, bs hzclient.BackupService) (*ClusterBackup, error) {
	ss.UpdateMembers(context.TODO())

	status := ss.GetStatus()
	if status == nil {
		return nil, errBackupClientNoMembers
	}

	if status.MemberDataMap == nil {
		return nil, errBackupClientNoMembers
	}

	return &ClusterBackup{
		statusService: ss,
		backupService: bs,
		members:       status.MemberDataMap,
	}, nil
}

func (b *ClusterBackup) Start(ctx context.Context) error {
	// switch cluster to passive for the time of hot backup
	err := retryOnError(backupBackoff,
		func() error { return b.backupService.ChangeClusterState(ctx, codecTypes.ClusterStatePassive) })
	if err != nil {
		return err
	}

	// activate cluster after backup, silently ignore state change status
	defer retryOnError(backupBackoff, //nolint:errcheck
		func() error { return b.backupService.ChangeClusterState(ctx, codecTypes.ClusterStateActive) })

	return retryOnError(backupBackoff,
		func() error { return b.backupService.TriggerHotRestartBackup(ctx) })
}

func retryOnError(backoff wait.Backoff, fn func() error) error {
	return retry.OnError(backoff, func(error) bool { return true }, fn)
}

func (b *ClusterBackup) Cancel(ctx context.Context) error {
	var err error
	b.cancelOnce.Do(func() {
		err = b.backupService.InterruptHotRestartBackup(ctx)
	})
	return err
}

func (b *ClusterBackup) Members() []*MemberBackup {
	var mb []*MemberBackup
	for uuid, m := range b.members {
		mb = append(mb, &MemberBackup{
			statusService: b.statusService,
			Address:       m.Address,
			UUID:          uuid,
		})
	}
	return mb
}

type MemberBackup struct {
	statusService hzclient.StatusService

	UUID    hztypes.UUID
	Address string
}

var (
	errMemberBackupFailed      = errors.New("member backup failed")
	errMemberBackupStateFailed = errors.New("member backup state update failed")
	errMemberBackupNoTask      = errors.New("member backup state indicates no task started")
)

func (mb *MemberBackup) Wait(ctx context.Context) error {
	var n int
	for {
		state, err := mb.statusService.GetTimedMemberState(ctx, mb.UUID)
		if err != nil {
			return errMemberBackupStateFailed
		}

		s := state.TimedMemberState.MemberState.HotRestartState.BackupTaskState
		switch s {
		case "FAILURE":
			return errMemberBackupFailed
		case "SUCCESS":
			return nil
		case "NO_TASK":
			// sometimes it takes few seconds for task to start
			if n > 10 {
				return errMemberBackupNoTask
			}
			n++
		case "IN_PROGRESS":
			// expected, check status again (no return)
		default:
			return errors.New("backup unknown status: " + s)

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
