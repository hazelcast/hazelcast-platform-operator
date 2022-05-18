package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type Client struct {
	sync.Mutex
	client               *hazelcast.Client
	cancel               context.CancelFunc
	NamespacedName       types.NamespacedName
	Error                error
	Log                  logr.Logger
	Status               *Status
	triggerReconcileChan chan event.GenericEvent
	statusTicker         *StatusTicker
}

type Status struct {
	MemberMap               map[hztypes.UUID]*MemberData
	ClusterHotRestartStatus ClusterHotRestartStatus
}

type StatusTicker struct {
	ticker *time.Ticker
	done   chan bool
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

func (m MemberData) String() string {
	return fmt.Sprintf("%s:%s", m.Address, m.UUID)
}

func newMemberData(m cluster.MemberInfo) *MemberData {
	return &MemberData{
		Address:    m.Address.String(),
		UUID:       m.UUID.String(),
		Version:    fmt.Sprintf("%d.%d.%d", m.Version.Major, m.Version.Minor, m.Version.Patch),
		LiteMember: m.LiteMember,
	}
}

func (m *MemberData) enrichMemberData(s TimedMemberState) {
	m.Master = s.Master
	m.MemberState = s.MemberState.NodeState.State
	m.Partitions = int32(len(s.MemberPartitionState.Partitions))
	m.Name = s.MemberState.Name
}

func (s *StatusTicker) stop() {
	s.ticker.Stop()
	s.done <- true
}

var clients sync.Map

func GetClient(ns types.NamespacedName) (client *Client, ok bool) {
	if v, ok := clients.Load(ns); ok {
		return v.(*Client), true
	}
	return nil, false
}

func CreateClient(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, channel chan event.GenericEvent, l logr.Logger) {
	ns := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	if _, ok := clients.Load(ns); ok {
		return
	}
	config := buildConfig(h)
	c := newHazelcastClient(l, ns, channel)
	c.start(ctx, config)
	clients.Store(ns, c)
}

func ShutdownClient(ctx context.Context, ns types.NamespacedName) {
	if c, ok := clients.LoadAndDelete(ns); ok {
		c.(*Client).shutdown(ctx)
	}
}

func newHazelcastClient(l logr.Logger, n types.NamespacedName, channel chan event.GenericEvent) *Client {
	return &Client{
		NamespacedName:       n,
		Log:                  l,
		Status:               &Status{MemberMap: make(map[hztypes.UUID]*MemberData)},
		triggerReconcileChan: channel,
	}
}

func (c *Client) start(ctx context.Context, config hazelcast.Config) {
	config.Cluster.ConnectionStrategy.Timeout = hztypes.Duration(0)
	config.Cluster.ConnectionStrategy.ReconnectMode = cluster.ReconnectModeOn
	config.Cluster.ConnectionStrategy.Retry = cluster.ConnectionRetryConfig{
		InitialBackoff: hztypes.Duration(1 * time.Second),
		MaxBackoff:     hztypes.Duration(10 * time.Second),
		Jitter:         0.25,
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func(ctx context.Context) {
		c.initHzClient(ctx, config)
	}(ctx)
	c.statusTicker = &StatusTicker{
		ticker: time.NewTicker(10 * time.Second),
		done:   make(chan bool),
	}

	go func(ctx context.Context, s *StatusTicker) {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				c.updateMembers(ctx)
				c.triggerReconcile()
			}
		}
	}(ctx, c.statusTicker)
}

func (c *Client) initHzClient(ctx context.Context, config hazelcast.Config) {
	hzClient, err := hazelcast.StartNewClientWithConfig(ctx, config)
	c.Lock()
	defer c.Unlock()
	if err != nil {
		// Ignoring the connection error and just logging as it is expected for Operator that in some scenarios it cannot access the HZ cluster
		c.Log.Info("Cannot connect to Hazelcast cluster. Some features might not be available.", "Reason", err.Error())
		c.Error = err
	} else {
		c.client = hzClient
	}
}

func (c *Client) shutdown(ctx context.Context) {
	if c.cancel != nil {
		c.cancel()
	}

	c.Lock()
	defer c.Unlock()

	if c.client == nil {
		return
	}
	if err := c.client.Shutdown(ctx); err != nil {
		c.Log.Error(err, "Problem occurred while shutting down the client connection")
	}
	if c.statusTicker != nil {
		c.statusTicker.stop()
	}
}

func (c *Client) triggerReconcile() {
	c.triggerReconcileChan <- event.GenericEvent{
		Object: &hazelcastv1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
			Namespace: c.NamespacedName.Namespace,
			Name:      c.NamespacedName.Name,
		}}}
}

func (c *Client) getTimedMemberState(ctx context.Context, uuid hztypes.UUID) *TimedMemberStateWrapper {
	jsonState, err := fetchTimedMemberState(ctx, c.client, uuid)
	if err != nil {
		c.Log.Error(err, "Fetching TimedMemberState failed.", "CR", c.NamespacedName)
		return nil
	}
	state := &TimedMemberStateWrapper{}
	err = json.Unmarshal([]byte(jsonState), state)
	if err != nil {
		c.Log.Error(err, "TimedMemberState json parsing failed.", "CR", c.NamespacedName, "JSON", jsonState)
		return nil
	}
	return state
}

func (c *Client) updateMembers(ctx context.Context) {
	if c.client == nil {
		return
	}
	c.Log.V(2).Info("Updating Hazelcast status", "CR", c.NamespacedName)
	hzInternalClient := hazelcast.NewClientInternal(c.client)

	activeMemberList := hzInternalClient.OrderedMembers()
	activeMembers := make(map[hztypes.UUID]*MemberData, len(activeMemberList))
	newClusterHotRestartStatus := &ClusterHotRestartStatus{}

	for _, memberInfo := range activeMemberList {
		activeMembers[memberInfo.UUID] = newMemberData(memberInfo)
		state := c.getTimedMemberState(ctx, memberInfo.UUID)
		if state != nil {
			activeMembers[memberInfo.UUID].enrichMemberData(state.TimedMemberState)
			newClusterHotRestartStatus = &state.TimedMemberState.MemberState.ClusterHotRestartStatus
		}
	}

	c.Lock()
	c.Status.MemberMap = activeMembers
	c.Status.ClusterHotRestartStatus = *newClusterHotRestartStatus
	c.Unlock()
}

func fetchTimedMemberState(ctx context.Context, client *hazelcast.Client, uuid hztypes.UUID) (string, error) {
	ci := hazelcast.NewClientInternal(client)
	req := codec.EncodeMCGetTimedMemberStateRequest()
	resp, err := ci.InvokeOnMember(ctx, req, uuid, nil)
	if err != nil {
		return "", fmt.Errorf("invoking: %w", err)
	}
	return codec.DecodeMCGetTimedMemberStateResponse(resp), nil
}

type TimedMemberStateWrapper struct {
	TimedMemberState TimedMemberState `json:"timedMemberState"`
}

type TimedMemberState struct {
	MemberState          MemberState          `json:"memberState"`
	MemberPartitionState MemberPartitionState `json:"memberPartitionState"`
	Master               bool                 `json:"master"`
}

type MemberState struct {
	Address                 string                  `json:"address"`
	Uuid                    string                  `json:"uuid"`
	Name                    string                  `json:"name"`
	NodeState               NodeState               `json:"nodeState"`
	HotRestartState         HotRestartState         `json:"hotRestartState"`
	ClusterHotRestartStatus ClusterHotRestartStatus `json:"clusterHotRestartStatus"`
}

type NodeState struct {
	State         string `json:"nodeState"`
	MemberVersion string `json:"memberVersion"`
}

type MemberPartitionState struct {
	Partitions []int32 `json:"partitions"`
}

type HotRestartState struct {
	BackupTaskState     string `json:"backupTaskState"`
	BackupTaskCompleted int32  `json:"backupTaskCompleted"`
	BackupTaskTotal     int32  `json:"backupTaskTotal"`
	IsHotBackupEnabled  bool   `json:"isHotBackupEnabled"`
	BackupDirectory     string `json:"backupDirectory"`
}

type ClusterHotRestartStatus struct {
	HotRestartStatus              string `json:"hotRestartStatus"`
	RemainingValidationTimeMillis int64  `json:"remainingValidationTimeMillis"`
	RemainingDataLoadTimeMillis   int64  `json:"remainingDataLoadTimeMillis"`
}

func (c ClusterHotRestartStatus) remainingValidationTimeSec() int64 {
	return int64((time.Duration(c.RemainingValidationTimeMillis) * time.Millisecond).Seconds())
}

func (c ClusterHotRestartStatus) remainingDataLoadTimeSec() int64 {
	return int64((time.Duration(c.RemainingDataLoadTimeMillis) * time.Millisecond).Seconds())
}

func (c ClusterHotRestartStatus) restoreState() hazelcastv1alpha1.RestoreState {
	switch c.HotRestartStatus {
	case "SUCCEEDED":
		return hazelcastv1alpha1.RestoreSucceeded
	case "IN_PROGRESS":
		return hazelcastv1alpha1.RestoreInProgress
	case "FAILED":
		return hazelcastv1alpha1.RestoreFailed
	default:
		return hazelcastv1alpha1.RestoreUnknown
	}
}
