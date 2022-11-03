package client

import (
	"context"
	"fmt"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type BackupService struct {
	Client ClientI
}

func NewBackupService(cl ClientI) *BackupService {
	return &BackupService{
		Client: cl,
	}
}

func (bs *BackupService) ChangeClusterState(ctx context.Context, newState codecTypes.ClusterState) error {
	req := codec.EncodeMCChangeClusterStateRequest(newState)
	_, err := bs.Client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("invoking: %w", err)
	}
	return nil
}

func (bs *BackupService) TriggerHotRestartBackup(ctx context.Context) error {
	req := codec.EncodeMCTriggerHotRestartBackupRequest()
	_, err := bs.Client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("invoking: %w", err)
	}
	return nil
}

func (bs *BackupService) InterruptHotRestartBackup(ctx context.Context) error {
	req := codec.EncodeMCInterruptHotRestartBackupRequest()
	_, err := bs.Client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("invoking: %w", err)
	}
	return nil
}
