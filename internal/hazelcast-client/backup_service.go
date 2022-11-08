package client

import (
	"context"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type BackupService interface {
	ChangeClusterState(ctx context.Context, newState codecTypes.ClusterState) error
	TriggerHotRestartBackup(ctx context.Context) error
	InterruptHotRestartBackup(ctx context.Context) error
}

type HzBackupService struct {
	client Client
}

func NewBackupService(cl Client) *HzBackupService {
	return &HzBackupService{
		client: cl,
	}
}

func (bs *HzBackupService) ChangeClusterState(ctx context.Context, newState codecTypes.ClusterState) error {
	req := codec.EncodeMCChangeClusterStateRequest(newState)
	_, err := bs.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return err
	}
	return nil
}

func (bs *HzBackupService) TriggerHotRestartBackup(ctx context.Context) error {
	req := codec.EncodeMCTriggerHotRestartBackupRequest()
	_, err := bs.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return err
	}
	return nil
}

func (bs *HzBackupService) InterruptHotRestartBackup(ctx context.Context) error {
	req := codec.EncodeMCInterruptHotRestartBackupRequest()
	_, err := bs.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return err
	}
	return nil
}
