package backup

import (
	"context"
	"errors"
	"sync"
	"time"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type ClusterBackup struct {
	statusService *hzclient.StatusService
	backupService *hzclient.BackupService
	members       map[hztypes.UUID]*hzclient.MemberData
	cancelOnce    sync.Once
}

var (
	errBackupClientNoMembers = errors.New("client couldnt connect to members")
)

func NewClusterBackup(ss *hzclient.StatusService, bs *hzclient.BackupService) (*ClusterBackup, error) {
	ss.UpdateMembers(context.TODO())

	if ss.Status == nil {
		return nil, errBackupClientNoMembers
	}

	if ss.Status.MemberMap == nil {
		return nil, errBackupClientNoMembers
	}

	return &ClusterBackup{
		statusService: ss,
		backupService: bs,
	}, nil
}

func (b *ClusterBackup) Start(ctx context.Context) error {
	// switch cluster to passive for the time of hot backup
	err := b.backupService.ChangeClusterState(ctx, codecTypes.ClusterStatePassive)
	if err != nil {
		return err
	}

	// activate cluster after backup, silently ignore state change status
	defer b.backupService.ChangeClusterState(ctx, codecTypes.ClusterStateActive) //nolint:errcheck

	return b.backupService.TriggerHotRestartBackup(ctx)
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
	statusService *hzclient.StatusService

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
