//go:build unittest
// +build unittest

// This file is used for unit tests

package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hzlogger "github.com/hazelcast/hazelcast-go-client/logger"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

const (
	localUrl  = "127.0.0.1:8000"
	AgentPort = 8443
)

func BuildConfig(h *hazelcastv1alpha1.Hazelcast, pool *x509.CertPool, cert *tls.Certificate, logger hzlogger.Logger) hazelcast.Config {
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
	cc.Unisocket = true
	return config
}

func RestUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("http://%s", HazelcastUrl(h))
}

func HazelcastUrl(_ *hazelcastv1alpha1.Hazelcast) string {
	return localUrl
}

func AgentUrl(host string) string {
	return fmt.Sprintf("https://%s:%d", host, AgentPort)
}
