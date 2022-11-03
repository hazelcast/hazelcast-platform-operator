package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type StatusService struct {
	client ClientI
	sync.Mutex
	cancel               context.CancelFunc
	NamespacedName       types.NamespacedName
	Log                  logr.Logger
	Status               *Status
	triggerReconcileChan chan event.GenericEvent
	statusTicker         *StatusTicker
}

func newMemberStatusService(cl ClientI, l logr.Logger, n types.NamespacedName, channel chan event.GenericEvent) *StatusService {
	return &StatusService{
		client:               cl,
		NamespacedName:       n,
		Log:                  l,
		Status:               &Status{MemberMap: make(map[hztypes.UUID]*MemberData)},
		triggerReconcileChan: channel,
	}
}

type Status struct {
	sync.Mutex
	MemberMap               map[hztypes.UUID]*MemberData
	ClusterHotRestartStatus codecTypes.ClusterHotRestartStatus
}

type MemberData struct {
	Address     string
	UUID        string
	Version     string
	LiteMember  bool
	MemberState string
	Master      bool
	Partitions  int32
	Name        string
}

func newMemberData(m cluster.MemberInfo) *MemberData {
	return &MemberData{
		Address:    m.Address.String(),
		UUID:       m.UUID.String(),
		Version:    fmt.Sprintf("%d.%d.%d", m.Version.Major, m.Version.Minor, m.Version.Patch),
		LiteMember: m.LiteMember,
	}
}

func (m *MemberData) enrichMemberData(s codecTypes.TimedMemberState) {
	m.Master = s.Master
	m.MemberState = s.MemberState.NodeState.State
	m.Partitions = int32(len(s.MemberPartitionState.Partitions))
	m.Name = s.MemberState.Name
}

func (m MemberData) String() string {
	return fmt.Sprintf("%s:%s", m.Address, m.UUID)
}

type StatusTicker struct {
	ticker *time.Ticker
	done   chan bool
}

func (s *StatusTicker) stop() {
	s.ticker.Stop()
	s.done <- true
}

func (ss *StatusService) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	ss.cancel = cancel

	ss.statusTicker = &StatusTicker{
		ticker: time.NewTicker(10 * time.Second),
		done:   make(chan bool),
	}

	go func(ctx context.Context, s *StatusTicker) {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				ss.UpdateMembers(ctx)
				ss.triggerReconcile()
			}
		}
	}(ctx, ss.statusTicker)
}

func (ss *StatusService) triggerReconcile() {
	ss.triggerReconcileChan <- event.GenericEvent{
		Object: &hazelcastv1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
			Namespace: ss.NamespacedName.Namespace,
			Name:      ss.NamespacedName.Name,
		}}}
}

func (ss *StatusService) UpdateMembers(ctx context.Context) {
	if ss.client == nil {
		return
	}
	ss.Log.V(2).Info("Updating Hazelcast status", "CR", ss.NamespacedName)

	activeMemberList := ss.client.OrderedMembers()
	activeMembers := make(map[hztypes.UUID]*MemberData, len(activeMemberList))
	newClusterHotRestartStatus := &codecTypes.ClusterHotRestartStatus{}

	for _, memberInfo := range activeMemberList {
		activeMembers[memberInfo.UUID] = newMemberData(memberInfo)
		state, err := fetchTimedMemberState(ctx, ss.client, memberInfo.UUID)
		if err != nil {
			ss.Log.V(2).Info("Error fetching timed member state", "CR", ss.NamespacedName, "error:", err)
		}
		activeMembers[memberInfo.UUID].enrichMemberData(state.TimedMemberState)
		newClusterHotRestartStatus = &state.TimedMemberState.MemberState.ClusterHotRestartStatus
	}

	ss.Status.Lock()
	ss.Status.MemberMap = activeMembers
	ss.Status.ClusterHotRestartStatus = *newClusterHotRestartStatus
	ss.Status.Unlock()
}

func (ss *StatusService) GetTimedMemberState(ctx context.Context, uuid hztypes.UUID) (*codecTypes.TimedMemberStateWrapper, error) {
	return fetchTimedMemberState(ctx, ss.client, uuid)
}

func fetchTimedMemberState(ctx context.Context, client ClientI, uuid hztypes.UUID) (*codecTypes.TimedMemberStateWrapper, error) {
	req := codec.EncodeMCGetTimedMemberStateRequest()
	resp, err := client.InvokeOnMember(ctx, req, uuid, nil)
	if err != nil {
		return nil, fmt.Errorf("invoking: %w", err)
	}
	jsonState := codec.DecodeMCGetTimedMemberStateResponse(resp)
	state, err := codec.DecodeTimedMemberStateJsonString(jsonState)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (ss *StatusService) Stop(ctx context.Context) {
	ss.Lock()
	defer ss.Unlock()

	ss.statusTicker.stop()
	ss.cancel()
}
