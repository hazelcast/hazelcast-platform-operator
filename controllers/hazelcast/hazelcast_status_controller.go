package hazelcast

import (
	"context"
	"sync"

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
	MemberMap            map[string]cluster.MemberInfo
	triggerReconcileChan chan event.GenericEvent
}

func NewHazelcastClient(l logr.Logger, n types.NamespacedName, channel chan event.GenericEvent) *HazelcastClient {
	return &HazelcastClient{
		NamespacedName:       n,
		Log:                  l,
		MemberMap:            make(map[string]cluster.MemberInfo),
		triggerReconcileChan: channel,
	}
}

func (c *HazelcastClient) start(ctx context.Context, config hazelcast.Config) {
	config.Cluster.ConnectionStrategy.Timeout = hztypes.Duration(0)
	config.Cluster.ConnectionStrategy.ReconnectMode = cluster.ReconnectModeOn
	config.Cluster.ConnectionStrategy.Retry = cluster.ConnectionRetryConfig{
		InitialBackoff: 1,
		MaxBackoff:     10,
		Jitter:         0.25,
	}

	ctx, cancel := context.WithCancel(ctx)
	c.Lock()
	c.cancel = cancel
	c.Unlock()

	go func(ctx context.Context) {
		hzClient, err := hazelcast.StartNewClientWithConfig(ctx, config)
		if err != nil {
			// Ignoring the connection error and just logging as it is expected for Operator that in some scenarios it cannot access the HZ cluster
			c.Log.Info("Cannot connect to Hazelcast cluster. Some features might not be available.", "Reason", err.Error())
		} else {
			c.Lock()
			c.client = hzClient
			c.Unlock()
		}
	}(ctx)
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
}

func getStatusUpdateListener(c *HazelcastClient) func(cluster.MembershipStateChanged) {
	return func(changed cluster.MembershipStateChanged) {
		if changed.State == cluster.MembershipStateAdded {
			c.Lock()
			c.MemberMap[changed.Member.UUID.String()] = changed.Member
			c.Unlock()
			c.Log.Info("Member is added", "member", changed.Member.String())
		} else if changed.State == cluster.MembershipStateRemoved {
			c.Lock()
			delete(c.MemberMap, changed.Member.UUID.String())
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
