package client

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	clientTypes "github.com/hazelcast/hazelcast-go-client/types"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type WanSyncMapRequest struct {
	hzResource  types.NamespacedName
	wanSync     string
	mapName     string
	wanName     string
	publisherId string
}

type WanSyncMapResponse struct {
	MapName       types.NamespacedName
	HazelcastName types.NamespacedName
	Event         codecTypes.MCEvent
}

type EventResponseFunc func(WanSyncMapResponse)

func NewWanSyncMapRequest(hzResource types.NamespacedName, wanSync, mapName, wanName, publisherId string) WanSyncMapRequest {
	return WanSyncMapRequest{
		hzResource:  hzResource,
		wanSync:     wanSync,
		mapName:     mapName,
		wanName:     wanName,
		publisherId: publisherId,
	}
}

func StartSyncJob(ctx context.Context, c Client, f EventResponseFunc, wsrs []WanSyncMapRequest, logger logr.Logger) {
	go doStartSyncJob(ctx, c, f, wsrs, logger)
}

func doStartSyncJob(ctx context.Context, c Client, f EventResponseFunc, wsrs []WanSyncMapRequest, logger logr.Logger) {
	for _, wsr := range wsrs {
		logger.V(util.DebugLevel).Info("Sending WAN Sync request.",
			"map", wsr.mapName, "hz", wsr.hzResource.Name, "publisherId", wsr.publisherId, "wan", wsr.wanName)
		f(WanSyncMapResponse{
			HazelcastName: wsr.hzResource,
			MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
			Event: codecTypes.MCEvent{
				Type: codecTypes.WanSyncStarted,
			}.WithMapName(wsr.mapName),
		})
		uuid, err := wanSyncMap(ctx, c, wsr)
		logger.V(util.DebugLevel).Info("WAN Sync request sent", "man", wsr.mapName, "uuid", uuid)
		if err != nil {
			logger.Error(err, "Error sending WAN Sync request", "map", wsr.mapName, "hz", wsr.hzResource.Name, "uuid", uuid)
			f(WanSyncMapResponse{
				HazelcastName: wsr.hzResource,
				MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
				Event: codecTypes.MCEvent{
					Type: codecTypes.WanSyncIgnored,
				}.WithReason(err.Error()).WithMapName(wsr.mapName),
			})
		}

		finishEvent := waitWanSyncToFinish(ctx, c, uuid, wsr, logger)
		f(WanSyncMapResponse{
			HazelcastName: wsr.hzResource,
			MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
			Event:         finishEvent,
		})
	}
}

func waitWanSyncToFinish(ctx context.Context, c Client, uuid clientTypes.UUID, wsr WanSyncMapRequest, logger logr.Logger) codecTypes.MCEvent {

	logger.V(util.DebugLevel).Info("Start polling MC events...", "uuid", uuid)
	members := c.OrderedMembers()
	event, done := pollMCEvents(ctx, c, members, uuid, logger)
	if done {
		return event
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	timeout := time.After(3 * time.Minute)
	for {
		select {
		case <-timeout:
			logger.Info("Timeout receiving events", "uuid", uuid)
			return codecTypes.MCEvent{Type: codecTypes.WanSyncIgnored}.
				WithReason(fmt.Errorf("timeout receiving events, uuid: %s", uuid.String()).Error()).
				WithMapName(wsr.mapName)
		case <-ticker.C:
			event, done = pollMCEvents(ctx, c, members, uuid, logger)
			if done {
				return event
			}
		}
	}
}

func pollMCEvents(
	ctx context.Context, c Client, members []cluster.MemberInfo, uuid clientTypes.UUID, logger logr.Logger) (codecTypes.MCEvent, bool) {

	for _, m := range members {
		request := codec.EncodeMCPollMCEventsRequest()
		resp, err := c.InvokeOnMember(ctx, request, m.UUID, nil)
		if err != nil {
			logger.Error(err, "Unable to retrieve MC events", "uuid", uuid)
			return codecTypes.MCEvent{}, false
		}
		events := codec.DecodeMCPollMCEventsResponse(resp)
		for _, event := range events {
			if event.UUID() == uuid.String() && event.Type.IsWanSync() {
				logger.V(util.DebugLevel).Info("Event received", "type", event.Type, "map", event.MapName(), "uuid", event.UUID())
				if !event.Type.IsInProgress() {
					logger.Info("Finished polling events", "map", event.MapName(), "type", event.Type, "uuid", uuid)
					return event, true
				}
			}
		}
	}
	return codecTypes.MCEvent{}, false
}

func wanSyncMap(ctx context.Context, c Client, sync WanSyncMapRequest) (clientTypes.UUID, error) {
	request := codec.EncodeMCWanSyncMapRequest(codecTypes.WanSyncRef{
		WanReplicationName: sync.wanName,
		WanPublisherId:     sync.publisherId,
		Type:               codecTypes.SingleMap,
		MapName:            sync.mapName,
	})
	uuid := clientTypes.UUID{}
	err := retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return err != nil
	}, func() error {
		resp, err := c.InvokeOnRandomTarget(ctx, request, nil)
		if err != nil {
			return err
		}
		uuid = codec.DecodeMCWanSyncMapResponse(resp)
		return nil
	})
	return uuid, err
}
