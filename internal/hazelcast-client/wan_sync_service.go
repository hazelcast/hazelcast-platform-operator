package client

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	clientTypes "github.com/hazelcast/hazelcast-go-client/types"
	"k8s.io/apimachinery/pkg/types"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type HzWanSyncService struct {
	client      Client
	wanSyncReqs []WanSyncMapRequest
}

func NewHzWanSyncService(c Client, reqs []WanSyncMapRequest) *HzWanSyncService {
	return &HzWanSyncService{
		client:      c,
		wanSyncReqs: reqs,
	}
}

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

func (ws *HzWanSyncService) StartSyncJob(ctx context.Context, f EventResponseFunc, logger logr.Logger) {
	go func() {
		for _, wsr := range ws.wanSyncReqs {
			logger.V(util.DebugLevel).Info("Sending WAN Sync request.",
				"map", wsr.mapName, "hz", wsr.hzResource.Name, "publisherId", wsr.publisherId, "wan", wsr.wanName)
			f(WanSyncMapResponse{
				HazelcastName: wsr.hzResource,
				MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
				Event: codecTypes.MCEvent{
					Type: codecTypes.WanSyncStarted,
				}.WithMapName(wsr.mapName),
			})
			uuid, err := ws.wanSyncMap(ctx, ws.client, wsr)
			if err != nil {
				logger.Error(err, "Error sending WAN Sync request", "map", wsr.mapName, "hz", wsr.hzResource.Name)
				f(WanSyncMapResponse{
					HazelcastName: wsr.hzResource,
					MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
					Event: codecTypes.MCEvent{
						Type: codecTypes.WanSyncIgnored,
					}.WithReason(err.Error()).WithMapName(wsr.mapName),
				})
			}

			finishEvent := ws.waitWanSyncToFinish(ctx, ws.client, uuid, logger)
			f(WanSyncMapResponse{
				HazelcastName: wsr.hzResource,
				MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
				Event:         finishEvent,
			})
		}
	}()
}

func (ws *HzWanSyncService) waitWanSyncToFinish(
	ctx context.Context, c Client, uuid clientTypes.UUID, logger logr.Logger) codecTypes.MCEvent {

	logger.V(util.DebugLevel).Info("Start polling MC events...")
	members := c.OrderedMembers()
	event, done := ws.pollMCEvents(ctx, c, members, uuid, logger)
	if done {
		return event
	}

	ticker := time.NewTicker(5 * time.Second)
	timeout := time.After(2 * time.Minute)
	for {
		select {
		case <-timeout:
			return codecTypes.MCEvent{Type: codecTypes.WanSyncIgnored}
		case <-ticker.C:
			event, done = ws.pollMCEvents(ctx, c, members, uuid, logger)
			if done {
				ticker.Stop()
				return event
			}
		}
	}
}

func (ws *HzWanSyncService) pollMCEvents(
	ctx context.Context, c Client, members []cluster.MemberInfo, uuid clientTypes.UUID, logger logr.Logger) (codecTypes.MCEvent, bool) {

	for _, m := range members {
		request := codec.EncodeMCPollMCEventsRequest()
		resp, err := c.InvokeOnMember(ctx, request, m.UUID, nil)
		if err != nil {
			logger.Error(err, "Unable to retrieve MC events")
			return codecTypes.MCEvent{}, false
		}
		events := codec.DecodeMCPollMCEventsResponse(resp)
		for _, event := range events {
			if event.UUID() == uuid.String() && event.Type.IsWanSync() {
				logger.V(util.DebugLevel).Info("Event received", "type", event.Type, "map", event.MapName(), "uuid", event.UUID())
				if !event.Type.IsInProgress() {
					logger.V(util.DebugLevel).Info("Finished polling events", "map", event.MapName(), "type", event.Type)
					return event, true
				}
			}
		}
	}
	return codecTypes.MCEvent{}, false
}

func (ws *HzWanSyncService) wanSyncMap(ctx context.Context, c Client, sync WanSyncMapRequest) (clientTypes.UUID, error) {
	request := codec.EncodeMCWanSyncMapRequest(codecTypes.WanSyncRef{
		WanReplicationName: sync.wanName,
		WanPublisherId:     sync.publisherId,
		Type:               codecTypes.SingleMap,
		MapName:            sync.mapName,
	})
	resp, err := c.InvokeOnRandomTarget(ctx, request, nil)
	if err != nil {
		return clientTypes.UUID{}, err
	}
	uuid := codec.DecodeMCWanSyncMapResponse(resp)
	return uuid, err
}
