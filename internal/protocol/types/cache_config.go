package types

import n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"

type CacheConfigs struct {
	Caches []CacheConfigInput `xml:"cache"`
}

type CacheConfigInput struct {
	Name                              string `xml:"name,attr"`
	BackupCount                       int32  `xml:"backup-count"`
	AsyncBackupCount                  int32  `xml:"async-backup-count"`
	StatisticsEnabled                 bool   `xml:"statistics-enabled"`
	ManagementEnabled                 bool
	ReadThrough                       bool
	WriteThrough                      bool
	MergePolicy                       string
	MergeBatchSize                    int32
	DisablePerEntryInvalidationEvents bool
	KeyType                           string `xml:"key-type"`
	ValueType                         string `xml:"value-type"`
	CacheLoaderFactory                string
	CacheWriterFactory                string
	CacheLoader                       string
	CacheWriter                       string
	InMemoryFormat                    InMemoryFormat
	SplitBrainProtectionName          string
	PartitionLostListenerConfigs      []ListenerConfigHolder
	ExpiryPolicyFactoryClassName      string
	TimedExpiryPolicyFactoryConfig    TimedExpiryPolicyFactoryConfig
	CacheEntryListeners               []ListenerConfigHolder
	EvictionConfig                    EvictionConfigHolder
	WanReplicationRef                 WanReplicationRef
	EventJournalConfig                EventJournalConfig
	HotRestartConfig                  HotRestartConfig
	MerkleTreeConfig                  MerkleTreeConfig
	DataPersistenceConfig             DataPersistenceConfig
}

// Default values are explicitly written for all fields that are not nullable
// even though most are the same with the default values in Go.
func DefaultCacheConfigInput() *CacheConfigInput {
	return &CacheConfigInput{
		BackupCount:                       n.DefaultCacheBackupCount,
		AsyncBackupCount:                  n.DefaultCacheAsyncBackupCount,
		StatisticsEnabled:                 n.DefaultCacheStatisticsEnabled,
		MergePolicy:                       n.DefaultCacheMergePolicy,
		MergeBatchSize:                    n.DefaultCacheMergeBatchSize,
		ManagementEnabled:                 n.DefaultCacheManagementEnabled,
		ReadThrough:                       n.DefaultCacheReadThrough,
		WriteThrough:                      n.DefaultCacheWriteThrough,
		DisablePerEntryInvalidationEvents: n.DefaultCacheDisablePerEntryInvalidationEvents,
		InMemoryFormat:                    InMemoryFormatBinary,
	}
}
