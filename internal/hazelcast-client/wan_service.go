package client

import (
	"context"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type WanService interface {
	AddBatchPublisherConfig(ctx context.Context, request *AddBatchPublisherRequest) error
	ChangeWanState(ctx context.Context, state codecTypes.WanReplicationState) error
	ClearWanQueue(ctx context.Context) error
}

type AddBatchPublisherRequest struct {
	TargetCluster         string
	Endpoints             string
	QueueCapacity         int32
	BatchSize             int32
	BatchMaxDelayMillis   int32
	ResponseTimeoutMillis int32
	AckType               hazelcastv1alpha1.AcknowledgementType
	QueueFullBehavior     hazelcastv1alpha1.FullBehaviorSetting
}

type HzWanService struct {
	client      Client
	name        string
	publisherId string
}

func NewWanService(cl Client, name, pubId string) *HzWanService {
	return &HzWanService{
		client:      cl,
		name:        name,
		publisherId: pubId,
	}
}

func (ws *HzWanService) AddBatchPublisherConfig(ctx context.Context, request *AddBatchPublisherRequest) error {

	req := codec.EncodeMCAddWanBatchPublisherConfigRequest(
		ws.name,
		request.TargetCluster,
		ws.publisherId,
		request.Endpoints,
		request.QueueCapacity,
		request.BatchSize,
		request.BatchMaxDelayMillis,
		request.ResponseTimeoutMillis,
		convertAckType(request.AckType),
		convertQueueBehavior(request.QueueFullBehavior),
	)

	_, err := ws.client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return err
	}

	return nil
}

func (ws *HzWanService) ChangeWanState(ctx context.Context, state codecTypes.WanReplicationState) error {
	req := codec.EncodeMCChangeWanReplicationStateRequest(
		ws.name,
		ws.publisherId,
		state,
	)

	for _, member := range ws.client.OrderedMembers() {
		_, err := ws.client.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ws *HzWanService) ClearWanQueue(ctx context.Context) error {
	req := codec.EncodeMCClearWanQueuesRequest(
		ws.name,
		ws.publisherId,
	)

	for _, member := range ws.client.OrderedMembers() {
		_, err := ws.client.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertAckType(ackType hazelcastv1alpha1.AcknowledgementType) int32 {
	switch ackType {
	case hazelcastv1alpha1.AckOnReceipt:
		return 0
	case hazelcastv1alpha1.AckOnOperationComplete:
		return 1
	default:
		return -1
	}
}

func convertQueueBehavior(behavior hazelcastv1alpha1.FullBehaviorSetting) int32 {
	switch behavior {
	case hazelcastv1alpha1.DiscardAfterMutation:
		return 0
	case hazelcastv1alpha1.ThrowException:
		return 1
	case hazelcastv1alpha1.ThrowExceptionOnlyIfReplicationActive:
		return 2
	default:
		return -1
	}
}
