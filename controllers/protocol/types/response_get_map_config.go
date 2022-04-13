package types

type MapConfig struct {
	InMemoryFormat    int32
	BackupCount       int32
	AsyncBackupCount  int32
	TimeToLiveSeconds int32
	MaxIdleSeconds    int32
	MaxSize           int32
	MaxSizePolicy     int32
	ReadBackupData    bool
	EvictionPolicy    int32
	MergePolicy       string
	Indexes           []IndexConfig
}

var InMemoryFormatEncode map[InMemoryFormat]int32 = map[InMemoryFormat]int32{
	InMemoryFormatBinary: 0,
	InMemoryFormatObject: 1,
	InMemoryFormatNative: 2,
}

var MaxSizePolicyEncode map[MaxSizePolicyType]int32 = map[MaxSizePolicyType]int32{
	MaxSizePolicyPerNode:                    0,
	MaxSizePolicyPerPartition:               1,
	MaxSizePolicyUsedHeapPercentage:         2,
	MaxSizePolicyUsedHeapSize:               3,
	MaxSizePolicyFreeHeapPercentage:         4,
	MaxSizePolicyFreeHeapSize:               5,
	MaxSizePolicyUsedNativeMemorySize:       6,
	MaxSizePolicyUsedNativeMemoryPercentage: 7,
	MaxSizePolicyFreeNativeMemorySize:       8,
	MaxSizePolicyFreeNativeMemoryPercentage: 9,
}

var EvictionPolicyTypeEncode map[EvictionPolicyType]int32 = map[EvictionPolicyType]int32{
	EvictionPolicyLRU:    0,
	EvictionPolicyLFU:    1,
	EvictionPolicyNone:   2,
	EvictionPolicyRandom: 3,
}
