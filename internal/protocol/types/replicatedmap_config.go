package types

import (
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type ReplicatedMapConfigs struct {
	ReplicatedMaps []ReplicatedMapConfig `xml:"replicatedmap"`
}

type ReplicatedMapConfig struct {
	Name              string `xml:"name,attr"`
	InMemoryFormat    string `xml:"in-memory-format"`
	AsyncFillup       bool   `xml:"async-fillup"`
	MergePolicy       string
	MergeBatchSize    int32
	StatisticsEnabled bool
	// nullable
	ListenerConfigs []ListenerConfigHolder
	// nullable
	SplitBrainProtectionName string
}

// Default values are explicitly written for all fields that are not nullable
// even though most are the same with the default values in Go.
func DefaultReplicatedMapConfigInput() *ReplicatedMapConfig {
	return &ReplicatedMapConfig{
		InMemoryFormat:    n.DefaultReplicatedMapInMemoryFormat,
		AsyncFillup:       n.DefaultReplicatedMapAsyncFillup,
		MergePolicy:       n.DefaultReplicatedMapMergePolicy,
		MergeBatchSize:    n.DefaultReplicatedMapMergeBatchSize,
		StatisticsEnabled: n.DefaultReplicatedMapStatisticsEnabled,
	}
}
