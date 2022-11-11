package backup

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
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
	client     *hzclient.Client
	members    map[hztypes.UUID]*hzclient.MemberData
	cancelOnce sync.Once
}

var (
	errBackupClientNotFound  = errors.New("client not found for hot backup CR")
	errBackupClientNoMembers = errors.New("client couldnt connect to members")
)

func NewClusterBackup(h *hazelcastv1alpha1.Hazelcast) (*ClusterBackup, error) {
	c, ok := hzclient.GetClient(types.NamespacedName{Namespace: h.Namespace, Name: h.Name})
	if !ok {
		return nil, errBackupClientNotFound
	}

	c.UpdateMembers(context.TODO())

	if c.Status == nil {
		return nil, errBackupClientNoMembers
	}

	if c.Status.MemberMap == nil {
		return nil, errBackupClientNoMembers
	}

	return &ClusterBackup{
		client:  c,
		members: c.Status.MemberMap,
	}, nil
}

func (b *ClusterBackup) Start(ctx context.Context) error {
	// switch cluster to passive for the time of hot backup
	err := retryOnError(backupBackoff,
		func() error { return b.client.ChangeClusterState(ctx, codecTypes.ClusterStatePassive) })
	if err != nil {
		return err
	}

	// activate cluster after backup, silently ignore state change status
	defer retryOnError(backupBackoff, //nolint:errcheck
		func() error { return b.client.ChangeClusterState(ctx, codecTypes.ClusterStateActive) })

	return retryOnError(backupBackoff,
		func() error { return b.client.TriggerHotRestartBackup(ctx) })
}

func retryOnError(backoff wait.Backoff, fn func() error) error {
	return retry.OnError(backoff, func(error) bool { return true }, fn)
}

func (b *ClusterBackup) Cancel(ctx context.Context) error {
	var err error
	b.cancelOnce.Do(func() {
		err = b.client.InterruptHotRestartBackup(ctx)
	})
	return err
}

func (b *ClusterBackup) Members() []*MemberBackup {
	var mb []*MemberBackup
	for uuid, m := range b.members {
		mb = append(mb, &MemberBackup{
			client:  b.client,
			Address: m.Address,
			UUID:    uuid,
		})
	}
	return mb
}

type MemberBackup struct {
	client *hzclient.Client

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
		state := mb.client.GetTimedMemberState(ctx, mb.UUID)
		if state == nil {
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
