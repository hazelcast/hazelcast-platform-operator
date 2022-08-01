package hazelcast

import (
	"context"
	"fmt"
	"github.com/hazelcast/hazelcast-go-client"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

type WanPublisherObject interface {
	metav1.Object
	WanPublisherConfig() *hazelcastv1alpha1.WanPublisherConfig
	PublisherId() string
	WanConfigName() string
}
type addBatchPublisherRequest struct {
	name                  string
	targetCluster         string
	publisherId           string
	endpoints             string
	queueCapacity         int32
	batchSize             int32
	batchMaxDelayMillis   int32
	responseTimeoutMillis int32
	ackType               int32
	queueFullBehavior     int32
}

type changeWanStateRequest struct {
	name        string
	publisherId string
	state       codecTypes.WanReplicationState
}

func hazelcastWanReplicationName(mapName string) string {
	return mapName + "-default"
}

func applyWanReplication(ctx context.Context, client *hazelcast.Client, wan WanPublisherObject) (string, error) {
	publisherId := wan.GetName() + "-" + rand.String(16)

	config := wan.WanPublisherConfig()
	req := &addBatchPublisherRequest{
		hazelcastWanReplicationName(config.MapResourceName),
		config.TargetClusterName,
		publisherId,
		config.Endpoints,
		config.Queue.Capacity,
		config.Batch.Size,
		config.Batch.MaximumDelay,
		config.Acknowledgement.Timeout,
		convertAckType(config.Acknowledgement.Type),
		convertQueueBehavior(config.Queue.FullBehavior),
	}

	err := addBatchPublisherConfig(ctx, client, req)
	if err != nil {
		return "", fmt.Errorf("failed to apply WAN configuration: %w", err)
	}
	return publisherId, nil
}

func addBatchPublisherConfig(
	ctx context.Context,
	client *hazelcast.Client,
	request *addBatchPublisherRequest,
) error {
	cliInt := hazelcast.NewClientInternal(client)

	req := codec.EncodeMCAddWanBatchPublisherConfigRequest(
		request.name,
		request.targetCluster,
		request.publisherId,
		request.endpoints,
		request.queueCapacity,
		request.batchSize,
		request.batchMaxDelayMillis,
		request.responseTimeoutMillis,
		request.ackType,
		request.queueFullBehavior,
	)

	_, err := cliInt.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return err
	}
	return nil
}

func stopWanReplication(
	ctx context.Context, client *hazelcast.Client, wan WanPublisherObject) error {

	log := util.GetLogger(ctx)
	if wan.PublisherId() == "" {
		log.V(util.DebugLevel).Info("publisherId is empty, will skip stopping WAN replication")
		return nil
	}

	req := &changeWanStateRequest{
		name:        hazelcastWanReplicationName(wan.WanPublisherConfig().MapResourceName),
		publisherId: wan.PublisherId(),
		state:       codecTypes.WanReplicationStateStopped,
	}
	return changeWanState(ctx, client, req)
}

func changeWanState(ctx context.Context, client *hazelcast.Client, request *changeWanStateRequest) error {
	cliInt := hazelcast.NewClientInternal(client)

	req := codec.EncodeMCChangeWanReplicationStateRequest(
		request.name,
		request.publisherId,
		request.state,
	)

	for _, member := range cliInt.OrderedMembers() {
		_, err := cliInt.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
