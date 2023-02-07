//go:build !unittest
// +build !unittest

package client

import (
	"fmt"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hzlogger "github.com/hazelcast/hazelcast-go-client/logger"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

const (
	AgentPort = 8443
)

func BuildConfig(h *hazelcastv1alpha1.Hazelcast, logger hzlogger.Logger) hazelcast.Config {
	config := hazelcast.Config{
		Logger: hzlogger.Config{
			CustomLogger: logger,
		},
		Cluster: cluster.Config{
			ConnectionStrategy: cluster.ConnectionStrategyConfig{
				Timeout:       hztypes.Duration(3 * time.Second),
				ReconnectMode: cluster.ReconnectModeOn,
				Retry: cluster.ConnectionRetryConfig{
					InitialBackoff: hztypes.Duration(200 * time.Millisecond),
					MaxBackoff:     hztypes.Duration(1 * time.Second),
					Jitter:         0.25,
				},
			},
		},
	}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses(HazelcastUrl(h))
	return config
}

func RestUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("http://%s", HazelcastUrl(h))
}

func HazelcastUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, n.DefaultHzPort)
}

func HazelcastUrlForWanConfig(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, 5710)
}

func AgentUrl(host string) string {
	return fmt.Sprintf("https://%s:%d", host, AgentPort)
}
