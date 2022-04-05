package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/codec"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type HazelcastClient struct {
	sync.Mutex
	client               *hazelcast.Client
	cancel               context.CancelFunc
	NamespacedName       types.NamespacedName
	Log                  logr.Logger
	MemberMap            map[hztypes.UUID]*MemberData
	triggerReconcileChan chan event.GenericEvent
	statusTicker         *StatusTicker
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

func NewHazelcastClient(l logr.Logger, n types.NamespacedName, channel chan event.GenericEvent) *HazelcastClient {
	return &HazelcastClient{
		NamespacedName:       n,
		Log:                  l,
		MemberMap:            make(map[hztypes.UUID]*MemberData),
		triggerReconcileChan: channel,
	}
}

func (c *HazelcastClient) start(ctx context.Context, config hazelcast.Config) {
	config.Cluster.ConnectionStrategy.Timeout = hztypes.Duration(0)
	config.Cluster.ConnectionStrategy.ReconnectMode = cluster.ReconnectModeOn
	config.Cluster.ConnectionStrategy.Retry = cluster.ConnectionRetryConfig{
		InitialBackoff: hztypes.Duration(1 * time.Second),
		MaxBackoff:     hztypes.Duration(10 * time.Second),
		Jitter:         0.25,
	}

	ctx, cancel := context.WithCancel(ctx)
	c.Lock()
	c.cancel = cancel
	c.Unlock()

	go func(ctx context.Context) {
		c.initHzClient(ctx, config)
	}(ctx)
	c.statusTicker = &StatusTicker{
		ticker: time.NewTicker(time.Minute),
		done:   make(chan bool),
	}

	go func(ctx context.Context, s *StatusTicker) {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				c.updateMemberStates(ctx)
			}
		}
	}(ctx, c.statusTicker)
}

func (c *HazelcastClient) initHzClient(ctx context.Context, config hazelcast.Config) {
	c.Lock()
	defer c.Unlock()
	hzClient, err := hazelcast.StartNewClientWithConfig(ctx, config)
	if err != nil {
		// Ignoring the connection error and just logging as it is expected for Operator that in some scenarios it cannot access the HZ cluster
		c.Log.Info("Cannot connect to Hazelcast cluster. Some features might not be available.", "Reason", err.Error())
	} else {
		c.client = hzClient
	}
}

func (c *HazelcastClient) shutdown(ctx context.Context) {
	c.Lock()
	defer c.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
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

func getStatusUpdateListener(ctx context.Context, c *HazelcastClient) func(cluster.MembershipStateChanged) {
	return func(changed cluster.MembershipStateChanged) {
		if changed.State == cluster.MembershipStateAdded {
			c.Lock()
			m := newMemberData(changed.Member)
			c.enrichMemberByUuid(ctx, changed.Member.UUID, m)
			c.MemberMap[changed.Member.UUID] = m
			c.Unlock()
			c.Log.Info("Member is added", "member", changed.Member.String())
		} else if changed.State == cluster.MembershipStateRemoved {
			c.Lock()
			delete(c.MemberMap, changed.Member.UUID)
			c.Unlock()
			c.Log.Info("Member is deleted", "member", changed.Member.String())
		}
		c.triggerReconcile()
	}
}

func (c *HazelcastClient) triggerReconcile() {
	c.triggerReconcileChan <- event.GenericEvent{
		Object: &v1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
			Namespace: c.NamespacedName.Namespace,
			Name:      c.NamespacedName.Name,
		}}}
}

func (c *HazelcastClient) updateMemberStates(ctx context.Context) {
	if c.client == nil {
		return
	}
	c.Log.V(2).Info("Updating Hazelcast status", "CR", c.NamespacedName)
	for uuid, m := range c.MemberMap {
		c.enrichMemberByUuid(ctx, uuid, m)
	}
	c.triggerReconcile()
}

func (c *HazelcastClient) enrichMemberByUuid(ctx context.Context, uuid hztypes.UUID, m *MemberData) {
	jsonState, err := fetchTimedMemberState(ctx, c.client, uuid)
	if err != nil {
		c.Log.Error(err, "Fetching TimedMemberState failed.", "CR", c.NamespacedName)
	}
	state := &TimedMemberStateWrapper{}
	err = json.Unmarshal([]byte(jsonState), state)
	if err != nil {
		c.Log.Error(err, "TimedMemberState json parsing failed.", "CR", c.NamespacedName, "JSON", jsonState)
		return
	}
	m.enrichMemberData(state.TimedMemberState)
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
	Address   string    `json:"address"`
	Uuid      string    `json:"uuid"`
	Name      string    `json:"name"`
	NodeState NodeState `json:"nodeState"`
}

type NodeState struct {
	State         string `json:"nodeState"`
	MemberVersion string `json:"memberVersion"`
}

type MemberPartitionState struct {
	Partitions []int32 `json:"partitions"`
}
