package client

import (
	"context"
	"fmt"
	"sync"
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

type WanSyncService struct {
	m              sync.Mutex
	clientRegistry ClientRegistry
	wanSyncs       sync.Map
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

func NewWanSyncService(registry ClientRegistry) *WanSyncService {
	return &WanSyncService{
		clientRegistry: registry,
	}
}

func NewWanSyncMapRequest(hzResource types.NamespacedName, wanSync, mapName, wanName, publisherId string) WanSyncMapRequest {
	return WanSyncMapRequest{
		hzResource:  hzResource,
		wanSync:     wanSync,
		mapName:     mapName,
		wanName:     wanName,
		publisherId: publisherId,
	}
}

func (s *WanSyncService) AddWanSyncRequests(ws types.NamespacedName, wsrs []WanSyncMapRequest) {
	s.wanSyncs.LoadOrStore(ws, wsrs)
}

func (s *WanSyncService) StartSyncJob(ctx context.Context, f EventResponseFunc, logger logr.Logger) {
	go func() {
		s.m.Lock()
		defer s.m.Unlock()
		s.wanSyncs.Range(func(key, value any) bool {
			wsrs := value.([]WanSyncMapRequest)
			s.doStartSyncJob(ctx, f, wsrs, logger)
			s.wanSyncs.Delete(key)
			return true
		})
	}()

}

func (s *WanSyncService) doStartSyncJob(ctx context.Context, f EventResponseFunc, wsrs []WanSyncMapRequest, logger logr.Logger) {
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
		c, err := s.clientRegistry.GetOrCreate(ctx, wsr.hzResource)
		if err != nil {
			logger.Error(err, "Error creating Hazelcast client for WAN Sync request", "map", wsr.mapName, "hz", wsr.hzResource.Name)
			f(WanSyncMapResponse{
				HazelcastName: wsr.hzResource,
				MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
				Event: codecTypes.MCEvent{
					Type: codecTypes.WanSyncIgnored,
				}.WithReason(err.Error()).WithMapName(wsr.mapName),
			})
			continue
		}
		uuid, err := s.wanSyncMap(ctx, c, wsr)
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
			continue
		}

		finishEvent := s.waitWanSyncToFinish(ctx, c, uuid, wsr, logger)
		f(WanSyncMapResponse{
			HazelcastName: wsr.hzResource,
			MapName:       types.NamespacedName{Namespace: wsr.hzResource.Namespace, Name: wsr.mapName},
			Event:         finishEvent,
		})
	}
}

func (s *WanSyncService) waitWanSyncToFinish(ctx context.Context, c Client, uuid clientTypes.UUID, wsr WanSyncMapRequest, logger logr.Logger) codecTypes.MCEvent {
	var wg sync.WaitGroup
	logger.V(util.DebugLevel).Info("Start polling MC events...", "uuid", uuid)
	members := c.OrderedMembers()
	eventsCh := make(chan codecTypes.MCEvent, len(members))
	for _, member := range members {
		logger.V(util.DebugLevel).Info("Polling from", "member", member.String(), "uuid", uuid)
		wg.Add(1)
		go func(member cluster.MemberInfo, wg *sync.WaitGroup, ch chan<- codecTypes.MCEvent) {
			defer wg.Done()
			event, err := s.waitMemberFinishEvent(ctx, c, member, uuid, logger)
			if err != nil {
				ch <- codecTypes.MCEvent{Type: codecTypes.WanSyncIgnored}.
					WithReason(err.Error()).
					WithMapName(wsr.mapName)
			} else {
				ch <- event
			}
		}(member, &wg, eventsCh)
	}

	wg.Wait()
	close(eventsCh)

	var finishEvent codecTypes.MCEvent
	for event := range eventsCh {
		if event.Type.IsError() {
			return event
		}
		finishEvent = event
	}
	return finishEvent
}

func (s *WanSyncService) waitMemberFinishEvent(
	ctx context.Context, c Client, member cluster.MemberInfo, uuid clientTypes.UUID, logger logr.Logger) (codecTypes.MCEvent, error) {
	//ticker := time.NewTicker(5 * time.Second)
	//defer ticker.Stop()

	errorCount := 0
	noEventsCount := 0
	for {
		request := codec.EncodeMCPollMCEventsRequest()
		resp, err := c.InvokeOnMember(ctx, request, member.UUID, nil)
		if err != nil {
			logger.Error(err, "Unable to retrieve MC events", "uuid", uuid)
			errorCount++
			if errorCount > 3 {
				return codecTypes.MCEvent{}, err
			}
			continue
		} else {
			errorCount = 0
		}
		events := filterWanSyncEvents(codec.DecodeMCPollMCEventsResponse(resp), uuid, logger)
		if len(events) == 0 {
			noEventsCount++
			if noEventsCount > 24 { //No events for 2 minutes (120 sec)
				break
			}
		} else {
			noEventsCount = 0
		}
		for _, event := range events {
			logger.V(util.DebugLevel).Info("Event received", "type", event.Type, "map", event.MapName(), "uuid", event.UUID())
			if !event.Type.IsInProgress() {
				logger.Info("Finished polling events", "map", event.MapName(), "type", event.Type, "uuid", uuid)
				return event, nil
			}
		}
		time.Sleep(5 * time.Second)
	}
	return codecTypes.MCEvent{}, fmt.Errorf("could not find MC events for UUID %s", uuid.String())
}

func filterWanSyncEvents(events []codecTypes.MCEvent, uuid clientTypes.UUID, logger logr.Logger) []codecTypes.MCEvent {
	logger.V(util.DebugLevel).Info("Filtering out events", "eventsCount", len(events), "uuid", uuid.String())
	var fEvents []codecTypes.MCEvent
	for _, e := range events {
		logger.V(util.DebugLevel).Info("Filtering out event.", "type", e.Type, "map", e.MapName(), "uuid", e.UUID())
		if e.Type.IsWanSync() && e.UUID() == uuid.String() {
			fEvents = append(fEvents, e)
		}
	}
	return fEvents
}

func (s *WanSyncService) wanSyncMap(ctx context.Context, c Client, sync WanSyncMapRequest) (clientTypes.UUID, error) {
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
