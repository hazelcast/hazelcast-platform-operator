package types

import (
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type MultiMapConfigs struct {
	MultiMaps []MultiMapConfig `xml:"multimap"`
}

type MultiMapConfig struct {
	Name              string `xml:"name,attr"`
	CollectionType    string `xml:"value-collection-type"`
	BackupCount       int32  `xml:"backup-count"`
	AsyncBackupCount  int32
	Binary            bool `xml:"binary"`
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
func DefaultMultiMapConfigInput() *MultiMapConfig {
	return &MultiMapConfig{
		CollectionType:    n.DefaultClusterName,
		BackupCount:       n.DefaultMapBackupCount,
		AsyncBackupCount:  n.DefaultMultiMapAsyncBackupCount,
		Binary:            n.DefaultMultiMapBinary,
		MergePolicy:       n.DefaultMultiMapMergePolicy,
		MergeBatchSize:    n.DefaultMultiMapMergeBatchSize,
		StatisticsEnabled: n.DefaultMultiMapStatisticsEnabled,
	}
}
