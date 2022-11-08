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

type StatusService interface {
	Start()
	UpdateMembers(ctx context.Context)
	GetStatus() *Status
	GetTimedMemberState(ctx context.Context, uuid hztypes.UUID) (*codecTypes.TimedMemberStateWrapper, error)
	Stop()
}

type Status struct {
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

type HzStatusService struct {
	sync.Mutex
	client               Client
	cancel               context.CancelFunc
	namespacedName       types.NamespacedName
	log                  logr.Logger
	status               *Status
	statusLock           sync.Mutex
	triggerReconcileChan chan event.GenericEvent
	statusTicker         *StatusTicker
}

type StatusTicker struct {
	ticker *time.Ticker
	done   chan bool
}

func (s *StatusTicker) stop() {
	s.ticker.Stop()
	s.done <- true
}

func NewStatusService(n types.NamespacedName, cl Client, l logr.Logger, channel chan event.GenericEvent) *HzStatusService {
	return &HzStatusService{
		client:               cl,
		namespacedName:       n,
		log:                  l,
		status:               &Status{MemberMap: make(map[hztypes.UUID]*MemberData)},
		triggerReconcileChan: channel,
	}
}

func (ss *HzStatusService) Start() {
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

func (ss *HzStatusService) triggerReconcile() {
	ss.triggerReconcileChan <- event.GenericEvent{
		Object: &hazelcastv1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
			Namespace: ss.namespacedName.Namespace,
			Name:      ss.namespacedName.Name,
		}}}
}

func (ss *HzStatusService) GetStatus() *Status {
	return ss.status
}

func (ss *HzStatusService) UpdateMembers(ctx context.Context) {
	if ss.client == nil {
		return
	}
	ss.log.V(2).Info("Updating Hazelcast status", "CR", ss.namespacedName)

	activeMemberList := ss.client.OrderedMembers()
	activeMembers := make(map[hztypes.UUID]*MemberData, len(activeMemberList))
	newClusterHotRestartStatus := &codecTypes.ClusterHotRestartStatus{}

	for _, memberInfo := range activeMemberList {
		activeMembers[memberInfo.UUID] = newMemberData(memberInfo)
		state, err := fetchTimedMemberState(ctx, ss.client, memberInfo.UUID)
		if err != nil {
			ss.log.V(2).Info("Error fetching timed member state", "CR", ss.namespacedName, "error:", err)
		}
		activeMembers[memberInfo.UUID].enrichMemberData(state.TimedMemberState)
		newClusterHotRestartStatus = &state.TimedMemberState.MemberState.ClusterHotRestartStatus
	}

	ss.statusLock.Lock()
	ss.status.MemberMap = activeMembers
	ss.status.ClusterHotRestartStatus = *newClusterHotRestartStatus
	ss.statusLock.Unlock()
}

func (ss *HzStatusService) GetTimedMemberState(ctx context.Context, uuid hztypes.UUID) (*codecTypes.TimedMemberStateWrapper, error) {
	return fetchTimedMemberState(ctx, ss.client, uuid)
}

func fetchTimedMemberState(ctx context.Context, client Client, uuid hztypes.UUID) (*codecTypes.TimedMemberStateWrapper, error) {
	req := codec.EncodeMCGetTimedMemberStateRequest()
	resp, err := client.InvokeOnMember(ctx, req, uuid, nil)
	if err != nil {
		return nil, err
	}
	jsonState := codec.DecodeMCGetTimedMemberStateResponse(resp)
	state, err := codec.DecodeTimedMemberStateJsonString(jsonState)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (ss *HzStatusService) Stop() {
	ss.Lock()
	defer ss.Unlock()

	ss.statusTicker.stop()
	ss.cancel()
}
