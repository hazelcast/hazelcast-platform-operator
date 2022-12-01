package client

import (
	"context"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type ClusterStateService interface {
	ClusterState(ctx context.Context) (codecTypes.ClusterState, error)
	ChangeClusterState(ctx context.Context, newState codecTypes.ClusterState) error
}

type HzClusterStateService struct {
	client Client
}

func NewClusterStateService(cl Client) *HzClusterStateService {
	return &HzClusterStateService{
		client: cl,
	}
}

func (s *HzClusterStateService) ClusterState(ctx context.Context) (codecTypes.ClusterState, error) {
	req := codec.EncodeMCGetClusterMetadataRequest()
	resp, err := s.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return -1, err
	}
	metadata := codec.DecodeMCGetClusterMetadataResponse(resp)
	return metadata.CurrentState, nil
}

func (bs *HzClusterStateService) ChangeClusterState(ctx context.Context, newState codecTypes.ClusterState) error {
	req := codec.EncodeMCChangeClusterStateRequest(newState)
	_, err := bs.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return err
	}
	return nil
}
