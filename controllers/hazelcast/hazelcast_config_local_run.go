//go:build localrun || unittest
// +build localrun unittest

// This file is used  for running operator locally and connect to the cluster that uses ExposeExternally feature

package hazelcast

import (
	"fmt"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/logger"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

const localUrl = "127.0.0.1:8000"

func buildConfig(h *hazelcastv1alpha1.Hazelcast) hazelcast.Config {
	config := hazelcast.Config{
		Logger: logger.Config{
			Level: logger.OffLevel,
		},
	}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses(hazelcastUrl(h))
	cc.Unisocket = true
	return config
}

func restUrl(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("http://%s", hazelcastUrl(h))
}

func hazelcastUrl(_ *hazelcastv1alpha1.Hazelcast) string {
	return localUrl
}
