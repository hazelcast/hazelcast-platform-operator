package hazelcast

import (
	"context"
	"sync"
	"sync/atomic"

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
	Client               atomic.Value
	NamespacedName       types.NamespacedName
	Log                  logr.Logger
	MemberMap            map[string]bool
	triggerReconcileChan chan event.GenericEvent
}

func NewHazelcastClient(l logr.Logger, n types.NamespacedName, channel chan event.GenericEvent) *HazelcastClient {
	return &HazelcastClient{
		NamespacedName:       n,
		Log:                  l,
		MemberMap:            make(map[string]bool),
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

	go func(ctx context.Context) {
		hzClient, err := hazelcast.StartNewClientWithConfig(ctx, config)
		if err != nil {
			// Ignoring the connection error and just logging as it is expected for Operator that in some scenarios it cannot access the HZ cluster
			c.Log.Info("Cannot connect to Hazelcast cluster. Some features might not be available.", "Reason", err.Error())
		} else {
			c.Client.Store(hzClient)
		}
	}(ctx)
}

func getStatusUpdateListener(hzClient *HazelcastClient) func(cluster.MembershipStateChanged) {
	return func(changed cluster.MembershipStateChanged) {
		if changed.State == cluster.MembershipStateAdded {
			hzClient.Lock()
			hzClient.MemberMap[changed.Member.String()] = true
			hzClient.Unlock()
		} else if changed.State == cluster.MembershipStateRemoved {
			hzClient.Lock()
			delete(hzClient.MemberMap, changed.Member.String())
			hzClient.Unlock()
		}
		hzClient.triggerReconcile()
	}
}

func (hzClient *HazelcastClient) triggerReconcile() {
	hzClient.triggerReconcileChan <- event.GenericEvent{
		Object: &v1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
			Namespace: hzClient.NamespacedName.Namespace,
			Name:      hzClient.NamespacedName.Name,
		}}}
}
