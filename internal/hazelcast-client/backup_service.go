package client

import (
	"context"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type BackupService struct {
	Client Client
}

func NewBackupService(cl Client) *BackupService {
	return &BackupService{
		Client: cl,
	}
}

func (bs *BackupService) ChangeClusterState(ctx context.Context, newState codecTypes.ClusterState) error {
	req := codec.EncodeMCChangeClusterStateRequest(newState)
	_, err := bs.Client.InvokeOnMember(ctx, req, hztypes.UUID{}, nil)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BackupService) TriggerHotRestartBackup(ctx context.Context) error {
	req := codec.EncodeMCTriggerHotRestartBackupRequest()
	_, err := bs.Client.InvokeOnMember(ctx, req, hztypes.UUID{}, nil)
	if err != nil {
		return err
	}
	return nil
}

func (bs *BackupService) InterruptHotRestartBackup(ctx context.Context) error {
	req := codec.EncodeMCInterruptHotRestartBackupRequest()
	_, err := bs.Client.InvokeOnMember(ctx, req, hztypes.UUID{}, nil)
	if err != nil {
		return err
	}
	return nil
}
