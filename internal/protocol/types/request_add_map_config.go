package types

import (
	iserialization "github.com/hazelcast/hazelcast-go-client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type MapConfigs struct {
	Maps []AddMapConfigInput `xml:"map"`
}

type AddMapConfigInput struct {
	Name              string `xml:"name,attr"`
	BackupCount       int32  `xml:"backup-count"`
	AsyncBackupCount  int32  `xml:"async-backup-count"`
	TimeToLiveSeconds int32  `xml:"time-to-live-seconds"`
	MaxIdleSeconds    int32  `xml:"max-idle-seconds"`
	// nullable
	EvictionConfig          EvictionConfigHolder
	ReadBackupData          bool
	CacheDeserializedValues string
	MergePolicy             string
	MergeBatchSize          int32
	InMemoryFormat          string `xml:"in-memory-format"`
	// nullable
	ListenerConfigs []ListenerConfigHolder
	// nullable
	PartitionLostListenerConfigs []ListenerConfigHolder
	StatisticsEnabled            bool
	// nullable
	SplitBrainProtectionName string
	// nullable
	MapStoreConfig MapStoreConfigHolder
	// nullable
	NearCacheConfig NearCacheConfigHolder
	// nullable
	WanReplicationRef WanReplicationRef
	// nullable
	IndexConfigs []IndexConfig
	// nullable
	AttributeConfigs []AttributeConfig
	// nullable
	QueryCacheConfigs []QueryCacheConfigHolder
	// nullable
	PartitioningStrategyClassName string
	// nullable
	PartitioningStrategyImplementation iserialization.Data
	// nullable
	HotRestartConfig HotRestartConfig
	// nullable
	EventJournalConfig EventJournalConfig
	// nullable
	MerkleTreeConfig      MerkleTreeConfig `xml:"merkle-tree"`
	MetadataPolicy        int32
	PerEntryStatsEnabled  bool
	TieredStoreConfig     TieredStoreConfig `xml:"tiered-store"`
	DataPersistenceConfig DataPersistenceConfig
}

// Default values are explicitly written for all fields that are not nullable
// even though most are the same with the default values in Go.
func DefaultAddMapConfigInput() *AddMapConfigInput {
	return &AddMapConfigInput{
		BackupCount:       n.DefaultMapBackupCount,
		AsyncBackupCount:  n.DefaultMapAsyncBackupCount,
		TimeToLiveSeconds: n.DefaultMapTimeToLiveSeconds,
		MaxIdleSeconds:    n.DefaultMapMaxIdleSeconds,
		// workaround for protocol definition and implementation discrepancy in core side
		EvictionConfig: EvictionConfigHolder{
			EvictionPolicy: n.DefaultMapEvictionPolicy,
			MaxSizePolicy:  n.DefaultMapMaxSizePolicy,
			Size:           n.DefaultMapMaxSize,
		},
		ReadBackupData:          false,
		CacheDeserializedValues: "INDEX_ONLY",
		MergePolicy:             "com.hazelcast.spi.merge.PutIfAbsentMergePolicy",
		MergeBatchSize:          int32(100),
		InMemoryFormat:          "BINARY",
		StatisticsEnabled:       true,
		// workaround for protocol definition and implementation discrepancy in core side
		HotRestartConfig: HotRestartConfig{
			Enabled: n.DefaultMapPersistenceEnabled,
			Fsync:   false,
		},
		MetadataPolicy:       0,
		PerEntryStatsEnabled: false,
		NearCacheConfig: NearCacheConfigHolder{
			EvictionConfigHolder: EvictionConfigHolder{},
		},
		TieredStoreConfig: TieredStoreConfig{
			Enabled: false,
			MemoryTierConfig: MemoryTierConfig{
				Capacity: Capacity{ // These values are the default values taken from Hazelcast doc.
					Value: int64(256),  // 256 MB
					Unit:  "MEGABYTES", // 2 refers MB.
				},
			},
			DiskTierConfig: DiskTierConfig{
				Enabled:    false,
				DeviceName: "default-tiered-store-device",
			},
		},
	}
}
