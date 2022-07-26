package backup

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/client"
	hzconfig "github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/config"
)

type ClusterBackup struct {
	client  *hzclient.Client
	service *rest.HazelcastService

	clusterName string
	members     map[hztypes.UUID]*hzclient.MemberData

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

	s, err := rest.NewHazelcastService(hzconfig.RestUrl(h))
	if err != nil {
		return nil, err
	}

	return &ClusterBackup{
		client:      c,
		service:     s,
		members:     c.Status.MemberMap,
		clusterName: h.Spec.ClusterName,
	}, nil
}

func (b *ClusterBackup) Start(ctx context.Context) error {
	// switch cluster to passive for the time of hot backup
	err := retryOnError(retry.DefaultRetry, func() error {
		_, _, err := b.service.ChangeState(ctx, b.clusterName, "PASSIVE")
		return err
	})
	if err != nil {
		return err
	}
	// activate cluster after backup, silently ignore state change status
	defer retryOnError(retry.DefaultRetry, func() error { //nolint:errcheck
		_, _, err := b.service.ChangeState(ctx, b.clusterName, "ACTIVE")
		return err
	})

	return retryOnError(retry.DefaultBackoff, func() error {
		_, err := b.service.HotBackup(ctx, b.clusterName)
		return err
	})
}

func (b *ClusterBackup) Cancel(ctx context.Context) error {
	var err error
	b.cancelOnce.Do(func() {
		_, err = b.service.HotBackupInterrupt(ctx, b.clusterName)
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

func retryOnError(backoff wait.Backoff, fn func() error) error {
	return retry.OnError(backoff, isHazelcastError, fn)
}

func isHazelcastError(err error) bool {
	switch err.(type) {
	// normal response but with failed status
	case *rest.HazelcastError:
		return true
	// hot backup sometimes fails with generic 500
	case *rest.ErrorResponse:
		return true
	// all other errors including connection errors should fail
	default:
		return false
	}
}
