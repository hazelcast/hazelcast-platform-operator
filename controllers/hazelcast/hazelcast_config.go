//go:build !localrun && !unittest
// +build !localrun,!unittest

package hazelcast

import (
	"fmt"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/logger"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func buildConfig(h *hazelcastv1alpha1.Hazelcast) hazelcast.Config {
	config := hazelcast.Config{
		Logger: logger.Config{
			Level: logger.OffLevel,
		},
	}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses(hazelcastUrl(h))
	return config
}

func restUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("http://%s", hazelcastUrl(h))
}

func hazelcastUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, n.DefaultHzPort)
}
