package types

import (
	iserialization "github.com/hazelcast/hazelcast-go-client"
)

type EventJournalConfig struct {
	Enabled           bool
	Capacity          int32
	TimeToLiveSeconds int32
}

type EvictionConfigHolder struct {
	Size                int32
	MaxSizePolicy       string
	EvictionPolicy      string
	ComparatorClassName string
	Comparator          iserialization.Data
}

type ListenerConfigHolder struct {
	ListenerType           int32
	ListenerImplementation iserialization.Data
	ClassName              string
	IncludeValue           bool
	Local                  bool
}

type MapStoreConfigHolder struct {
	Enabled               bool
	WriteCoalescing       bool
	WriteDelaySeconds     int32
	WriteBatchSize        int32
	ClassName             string
	Implementation        iserialization.Data
	FactoryClassName      string
	FactoryImplementation iserialization.Data
	Properties            map[string]string
	InitialLoadMode       string
}

type MerkleTreeConfig struct {
	Enabled    bool
	Depth      int32
	EnabledSet bool
}

type NearCacheConfigHolder struct {
	Name                 string
	InMemoryFormat       string
	SerializeKeys        bool
	InvalidateOnChange   bool
	TimeToLiveSeconds    int32
	MaxIdleSeconds       int32
	EvictionConfigHolder EvictionConfigHolder
	CacheLocalEntries    bool
	LocalUpdatePolicy    string
	PreloaderConfig      NearCachePreloaderConfig
}

type NearCachePreloaderConfig struct {
	Enabled                  bool
	Directory                string
	StoreInitialDelaySeconds int32
	StoreIntervalSeconds     int32
}

type WanReplicationRef struct {
	Name                 string
	MergePolicyClassName string
	Filters              []string
	RepublishingEnabled  bool
}

type IndexConfig struct {
	Name               string
	Type               IndexType
	Attributes         []string
	BitmapIndexOptions BitmapIndexOptions
}

// - From client code types/index.go
type IndexType int32

const (
	IndexTypeSorted IndexType = 0
	IndexTypeHash   IndexType = 1
	IndexTypeBitmap IndexType = 2
) // -

type BitmapIndexOptions struct {
	UniqueKey               string
	UniqueKeyTransformation UniqueKeyTransformation
}

// - From client code types/index.go
type UniqueKeyTransformation int32

const (
	UniqueKeyTransformationObject UniqueKeyTransformation = 0
	UniqueKeyTransformationLong   UniqueKeyTransformation = 1
	UniqueKeyTransformationRaw    UniqueKeyTransformation = 2
) // -

type AttributeConfig struct {
	Name               string
	ExtractorClassName string
}

type QueryCacheConfigHolder struct {
	BatchSize             int32
	BufferSize            int32
	DelaySeconds          int32
	IncludeValue          bool
	Populate              bool
	Coalesce              bool
	InMemoryFormat        string
	Name                  string
	PredicateConfigHolder PredicateConfigHolder
	EvictionConfigHolder  EvictionConfigHolder
	ListenerConfigs       []ListenerConfigHolder
	IndexConfigs          []IndexConfig
}

type PredicateConfigHolder struct {
	ClassName      string
	Sql            string
	Implementation iserialization.Data
}

type HotRestartConfig struct {
	Enabled bool
	Fsync   bool
}
