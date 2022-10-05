package types

type IndexType int32

const (
	IndexTypeSorted IndexType = 0
	IndexTypeHash   IndexType = 1
	IndexTypeBitmap IndexType = 2
)

type UniqueKeyTransformation int32

const (
	UniqueKeyTransformationObject UniqueKeyTransformation = 0
	UniqueKeyTransformationLong   UniqueKeyTransformation = 1
	UniqueKeyTransformationRaw    UniqueKeyTransformation = 2
)

// +kubebuilder:validation:Enum=PER_NODE;PER_PARTITION;USED_HEAP_SIZE;USED_HEAP_PERCENTAGE;FREE_HEAP_SIZE;FREE_HEAP_PERCENTAGE;USED_NATIVE_MEMORY_SIZE;USED_NATIVE_MEMORY_PERCENTAGE;FREE_NATIVE_MEMORY_SIZE;FREE_NATIVE_MEMORY_PERCENTAGE
type MaxSizePolicyType string

const (
	// Maximum number of map entries in each cluster member.
	// You cannot set the max-size to a value lower than the partition count (which is 271 by default).
	MaxSizePolicyPerNode MaxSizePolicyType = "PER_NODE"

	// Maximum number of map entries within each partition.
	MaxSizePolicyPerPartition MaxSizePolicyType = "PER_PARTITION"

	// Maximum used heap size percentage per map for each Hazelcast instance.
	// If, for example, JVM is configured to have 1000 MB and this value is 10, then the map entries will be evicted when used heap size
	// exceeds 100 MB. It does not work when "in-memory-format" is set to OBJECT.
	MaxSizePolicyUsedHeapPercentage MaxSizePolicyType = "USED_HEAP_PERCENTAGE"

	// Maximum used heap size in megabytes per map for each Hazelcast instance. It does not work when "in-memory-format" is set to OBJECT.
	MaxSizePolicyUsedHeapSize MaxSizePolicyType = "USED_HEAP_SIZE"

	// Minimum free heap size percentage for each Hazelcast instance. If, for example, JVM is configured to
	// have 1000 MB and this value is 10, then the map entries will be evicted when free heap size is below 100 MB.
	MaxSizePolicyFreeHeapPercentage MaxSizePolicyType = "FREE_HEAP_PERCENTAGE"

	// Minimum free heap size in megabytes for each Hazelcast instance.
	MaxSizePolicyFreeHeapSize MaxSizePolicyType = "FREE_HEAP_SIZE"

	// Maximum used native memory size in megabytes per map for each Hazelcast instance. It is available only in
	// Hazelcast Enterprise HD.
	MaxSizePolicyUsedNativeMemorySize MaxSizePolicyType = "USED_NATIVE_MEMORY_SIZE"

	// Maximum used native memory size percentage per map for each Hazelcast instance. It is available only in
	// Hazelcast Enterprise HD.
	MaxSizePolicyUsedNativeMemoryPercentage MaxSizePolicyType = "USED_NATIVE_MEMORY_PERCENTAGE"

	// Minimum free native memory size in megabytes for each Hazelcast instance. It is available only in
	// Hazelcast Enterprise HD.
	MaxSizePolicyFreeNativeMemorySize MaxSizePolicyType = "FREE_NATIVE_MEMORY_SIZE"

	// Minimum free native memory size percentage for each Hazelcast instance. It is available only in
	// Hazelcast Enterprise HD.
	MaxSizePolicyFreeNativeMemoryPercentage MaxSizePolicyType = "FREE_NATIVE_MEMORY_PERCENTAGE"
)

// +kubebuilder:validation:Enum=NONE;LRU;LFU;RANDOM
type EvictionPolicyType string

const (
	// Least recently used entries will be removed.
	EvictionPolicyLRU EvictionPolicyType = "LRU"

	// Least frequently used entries will be removed.
	EvictionPolicyLFU EvictionPolicyType = "LFU"

	// No eviction.
	EvictionPolicyNone EvictionPolicyType = "NONE"

	// Randomly selected entries will be removed.
	EvictionPolicyRandom EvictionPolicyType = "RANDOM"
)

type InMemoryFormat string

const (
	InMemoryFormatBinary InMemoryFormat = "BINARY"
	InMemoryFormatObject InMemoryFormat = "OBJECT"
	InMemoryFormatNative InMemoryFormat = "NATIVE"
)

type MetadataPolicy int32

const (
	MetadataPolicyCreateOnUpdate MetadataPolicy = 0
	MetadataPolicyOff            MetadataPolicy = 1
)

type CacheDeserializedValues string

const (
	CacheDeserializedValuesNever     CacheDeserializedValues = "NEVER"
	CacheDeserializedValuesIndexOnly CacheDeserializedValues = "INDEX_ONLY"
	CacheDeserializedValuesAlways    CacheDeserializedValues = "ALWAYS"
)

type ClusterState int32

const (
	ClusterStateActive ClusterState = iota
	ClusterStateNoMigration
	ClusterStateFrozen
	ClusterStatePassive
	ClusterStateInTransition
)
