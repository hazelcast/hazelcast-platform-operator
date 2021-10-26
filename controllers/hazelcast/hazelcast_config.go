//go:build !localrun
// +build !localrun

package hazelcast

import (
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"

	"github.com/hazelcast/hazelcast-go-client"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func buildConfig(h *hazelcastv1alpha1.Hazelcast) hazelcast.Config {
	config := hazelcast.Config{}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses(fmt.Sprintf("%s.%s.svc.cluster.local:%d", h.Name, h.Namespace, n.DefaultHzPort))
	return config
}
