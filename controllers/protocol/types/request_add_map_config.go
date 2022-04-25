package types

import (
	iserialization "github.com/hazelcast/hazelcast-go-client"

	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

type AddMapConfigInput struct {
	Name              string
	BackupCount       int32
	AsyncBackupCount  int32
	TimeToLiveSeconds int32
	MaxIdleSeconds    int32
	// nullable
	EvictionConfig          EvictionConfigHolder
	ReadBackupData          bool
	CacheDeserializedValues CacheDeserializedValues
	MergePolicy             string
	MergeBatchSize          int32
	InMemoryFormat          InMemoryFormat
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
	MerkleTreeConfig     MerkleTreeConfig
	MetadataPolicy       MetadataPolicy
	PerEntryStatsEnabled bool
}

// Default values are explicitly written for all fields that are not nullable
// even though most are the same with the default values in Go.
func DefaultAddMapConfigInput() *AddMapConfigInput {
	return &AddMapConfigInput{
		BackupCount:       n.DefaultMapBackupCount,
		AsyncBackupCount:  int32(0),
		TimeToLiveSeconds: n.DefaultMapTimeToLiveSeconds,
		MaxIdleSeconds:    n.DefaultMapMaxIdleSeconds,
		// workaround for protocol definition and implementation discrepancy in core side
		EvictionConfig: EvictionConfigHolder{
			EvictionPolicy: n.DefaultMapEvictionPolicy,
			MaxSizePolicy:  n.DefaultMapMaxSizePolicy,
			Size:           n.DefaultMapMaxSize,
		},
		ReadBackupData:          false,
		CacheDeserializedValues: CacheDeserializedValuesIndexOnly,
		MergePolicy:             "com.hazelcast.spi.merge.PutIfAbsentMergePolicy",
		MergeBatchSize:          int32(100),
		InMemoryFormat:          InMemoryFormatBinary,
		StatisticsEnabled:       true,
		// workaround for protocol definition and implementation discrepancy in core side
		HotRestartConfig: HotRestartConfig{
			IsDefined: true,
			Enabled:   n.DefaultMapPersistenceEnabled,
			Fsync:     false,
		},
		// workaround for protocol definition and implementation discrepancy in core side
		EventJournalConfig: EventJournalConfig{IsDefined: true, Enabled: false, Capacity: 1000},
		// workaround for protocol definition and implementation discrepancy in core side
		MerkleTreeConfig:     MerkleTreeConfig{IsDefined: true, Enabled: false, Depth: 2},
		MetadataPolicy:       MetadataPolicyCreateOnUpdate,
		PerEntryStatsEnabled: false,
	}
}
