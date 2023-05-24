//go:build !unittest
// +build !unittest

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
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

const (
	AgentPort = 8443
)

func BuildConfig(h *hazelcastv1alpha1.Hazelcast, pool *x509.CertPool, cert *tls.Certificate, logger hzlogger.Logger) hazelcast.Config {
	config := hazelcast.Config{
		Logger: hzlogger.Config{
			CustomLogger: logger,
		},
		Cluster: cluster.Config{
			ConnectionStrategy: cluster.ConnectionStrategyConfig{
				Timeout:       hztypes.Duration(120 * time.Second),
				ReconnectMode: cluster.ReconnectModeOn,
				Retry: cluster.ConnectionRetryConfig{
					InitialBackoff: hztypes.Duration(20 * time.Millisecond),
					MaxBackoff:     hztypes.Duration(30 * time.Second),
					Multiplier:     1.05,
					Jitter:         0.05,
				},
			},
		},
	}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses(HazelcastUrl(h))
	if pool != nil {
		cc.Network.SSL.Enabled = true
		tlsConfig := tls.Config{
			RootCAs:            pool,
			InsecureSkipVerify: true,
		}
		if cert != nil {
			tlsConfig.Certificates = []tls.Certificate{*cert}
		}
		cc.Network.SSL.SetTLSConfig(&tlsConfig)
	}
	return config
}

func RestUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, n.RestServerSocketPort)
}

func HazelcastUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, n.DefaultHzPort)
}

func AgentUrl(host string) string {
	return fmt.Sprintf("https://%s:%d", host, AgentPort)
}
