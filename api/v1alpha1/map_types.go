package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MapSpec defines the desired state of Hazelcast Map Config
type MapSpec struct {
	DataStructureSpec `json:",inline"`

	// Maximum time in seconds for each entry to stay in the map.
	// If it is not 0, entries that are older than this time and not updated for this time are evicted automatically.
	// It can be updated.
	// +kubebuilder:default:=0
	// +optional
	TimeToLiveSeconds int32 `json:"timeToLiveSeconds"`

	// Maximum time in seconds for each entry to stay idle in the map.
	// Entries that are idle for more than this time are evicted automatically.
	// It can be updated.
	// +kubebuilder:default:=0
	// +optional
	MaxIdleSeconds int32 `json:"maxIdleSeconds"`

	// Configuration for removing data from the map when it reaches its max size.
	// It can be updated.
	// +kubebuilder:default:={maxSize: 0, evictionPolicy: NONE, maxSizePolicy: PER_NODE}
	// +optional
	Eviction EvictionConfig `json:"eviction,omitempty"`

	// Indexes to be created for the map data.
	// You can learn more at https://docs.hazelcast.com/hazelcast/latest/query/indexing-maps.
	// It cannot be updated after map config is created successfully.
	// +optional
	Indexes []IndexConfig `json:"indexes,omitempty"`

	// When enabled, map data will be persisted.
	// It cannot be updated after map config is created successfully.
	// +kubebuilder:default:=false
	// +optional
	PersistenceEnabled bool `json:"persistenceEnabled"`

	// Configuration options when you want to load/store the map entries
	// from/to a persistent data store such as a relational database
	// You can learn more at https://docs.hazelcast.com/hazelcast/latest/data-structures/working-with-external-data
	// +optional
	MapStore *MapStoreConfig `json:"mapStore,omitempty"`

	// InMemoryFormat specifies in which format data will be stored in your map
	// +kubebuilder:default:=BINARY
	// +optional
	InMemoryFormat InMemoryFormatType `json:"inMemoryFormat,omitempty"`

	// EntryListeners contains the configuration for the map-level or entry-based events listeners
	// provided by the Hazelcast’s eventing framework.
	// You can learn more at https://docs.hazelcast.com/hazelcast/latest/events/object-events.
	// +optional
	EntryListeners []EntryListenerConfiguration `json:"entryListeners,omitempty"`

	// InMemoryFormat specifies near cache configuration for map
	// +optional
	NearCache *NearCache `json:"nearCache"`

	// EventJournal specifies event journal configuration for map
	// +optional
	EventJournal EventJournal `json:"eventJournal"`
}

type NearCache struct {
	// Name is name of the near cache
	// +kubebuilder:default:=default
	// +optional
	Name string `json:"name,omitempty"`

	// InMemoryFormat specifies in which format data will be stored in your near cache
	// +kubebuilder:default:=BINARY
	// +optional
	InMemoryFormat InMemoryFormatType `json:"inMemoryFormat,omitempty"`

	// InvalidateOnChange specifies whether the cached entries are evicted when the entries are updated or removed
	// +kubebuilder:default:=true
	// +optional
	InvalidateOnChange *bool `json:"invalidateOnChange,omitempty"`

	// TimeToLiveSeconds maximum number of seconds for each entry to stay in the Near Cache
	// +kubebuilder:default:=0
	// +optional
	TimeToLiveSeconds uint `json:"timeToLiveSeconds,omitempty"`

	// MaxIdleSeconds Maximum number of seconds each entry can stay in the Near Cache as untouched (not read)
	// +kubebuilder:default:=0
	// +optional
	MaxIdleSeconds uint `json:"maxIdleSeconds,omitempty"`

	// NearCacheEviction specifies the eviction behavior in Near Cache
	// +optional
	NearCacheEviction *NearCacheEviction `json:"eviction,omitempty"`

	// CacheLocalEntries specifies whether the local entries are cached
	// +kubebuilder:default:=true
	// +optional
	CacheLocalEntries *bool `json:"cacheLocalEntries,omitempty"`
}

type NearCacheEviction struct {
	// EvictionPolicy to be applied when near cache reaches its max size according to the max size policy.
	// +kubebuilder:default:="NONE"
	// +optional
	EvictionPolicy EvictionPolicyType `json:"evictionPolicy,omitempty"`

	// MaxSizePolicy for deciding if the maxSize is reached.
	// +kubebuilder:default:="ENTRY_COUNT"
	// +optional
	MaxSizePolicy MaxSizePolicyType `json:"maxSizePolicy,omitempty"`

	// Size is maximum size of the Near Cache used for max-size-policy
	// +optional
	Size uint32 `json:"size,omitempty"`
}

type EntryListenerConfiguration struct {
	// ClassName is the fully qualified name of the class that implements any of the Listener interface.
	// +kubebuilder:validation:MinLength:=1
	// +required
	ClassName string `json:"className"`

	// IncludeValues is an optional attribute that indicates whether the event will contain the map value.
	// Defaults to true.
	// +kubebuilder:default:=true
	// +optional
	IncludeValues *bool `json:"includeValues,omitempty"`

	// Local is an optional attribute that indicates whether the map on the local member can be listened to.
	// Defaults to false.
	// +kubebuilder:default:=false
	// +optional
	Local bool `json:"local"`
}

func (e *EntryListenerConfiguration) GetIncludedValue() bool {
	if e.IncludeValues == nil {
		return true
	}
	return *e.IncludeValues
}

type EvictionConfig struct {
	// Eviction policy to be applied when map reaches its max size according to the max size policy.
	// +kubebuilder:default:="NONE"
	// +optional
	EvictionPolicy EvictionPolicyType `json:"evictionPolicy,omitempty"`

	// Max size of the map.
	// +kubebuilder:default:=0
	// +optional
	MaxSize int32 `json:"maxSize"`

	// Policy for deciding if the maxSize is reached.
	// +kubebuilder:default:="PER_NODE"
	// +optional
	MaxSizePolicy MaxSizePolicyType `json:"maxSizePolicy,omitempty"`
}

// +kubebuilder:validation:Enum=PER_NODE;PER_PARTITION;USED_HEAP_SIZE;USED_HEAP_PERCENTAGE;FREE_HEAP_SIZE;FREE_HEAP_PERCENTAGE;USED_NATIVE_MEMORY_SIZE;USED_NATIVE_MEMORY_PERCENTAGE;FREE_NATIVE_MEMORY_SIZE;FREE_NATIVE_MEMORY_PERCENTAGE;ENTRY_COUNT
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

	// Maximum size based on the entry count in the Near Cache
	// Warning: This policy is specific to near cache.
	MaxSizePolicyEntryCount MaxSizePolicyType = "ENTRY_COUNT"
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

type IndexConfig struct {
	// Name of the index config.
	// +optional
	Name string `json:"name,omitempty"`

	// Type of the index. See https://docs.hazelcast.com/hazelcast/latest/query/indexing-maps#index-types
	// +required
	Type IndexType `json:"type"`

	// Attributes of the index.
	// +optional
	Attributes []string `json:"attributes,omitempty"`

	// Options for "BITMAP" index type. See https://docs.hazelcast.com/hazelcast/latest/query/indexing-maps#configuring-bitmap-indexes
	// +kubebuilder:default:={}
	// +optional
	BitmapIndexOptions *BitmapIndexOptionsConfig `json:"bitMapIndexOptions,omitempty"`
}

// +kubebuilder:validation:Enum=SORTED;HASH;BITMAP
type IndexType string

const (
	IndexTypeSorted IndexType = "SORTED"
	IndexTypeHash   IndexType = "HASH"
	IndexTypeBitmap IndexType = "BITMAP"
)

type BitmapIndexOptionsConfig struct {
	// +required
	UniqueKey string `json:"uniqueKey"`

	// +required
	UniqueKeyTransition UniqueKeyTransition `json:"uniqueKeyTransition"`
}

// +kubebuilder:validation:Enum=OBJECT;LONG;RAW
type UniqueKeyTransition string

const (
	UniqueKeyTransitionObject UniqueKeyTransition = "OBJECT"
	UniqueKeyTransitionLong   UniqueKeyTransition = "LONG"
	UniqueKeyTransitionRAW    UniqueKeyTransition = "RAW"
)

// MapStatus defines the observed state of Map
type MapStatus struct {
	// +optional
	State MapConfigState `json:"state,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// +optional
	MemberStatuses map[string]MapConfigState `json:"memberStatuses,omitempty"`
}

// +kubebuilder:validation:Enum=Success;Failed;Pending;Persisting;Terminating
type MapConfigState string

const (
	MapFailed  MapConfigState = "Failed"
	MapSuccess MapConfigState = "Success"
	MapPending MapConfigState = "Pending"
	// Map config is added into all members but waiting for map to be persistent into ConfigMap
	MapPersisting  MapConfigState = "Persisting"
	MapTerminating MapConfigState = "Terminating"
)

type MapStoreConfig struct {
	// Sets the initial entry loading mode.
	// +kubebuilder:default:=LAZY
	// +optional
	InitialMode InitialModeType `json:"initialMode,omitempty"`

	// Name of your class implementing MapLoader and/or MapStore interface.
	// +required
	ClassName string `json:"className"`

	// Number of seconds to delay the storing of entries.
	// +kubebuilder:default:0
	// +optional
	WriteDelaySeconds int32 `json:"writeDelaySeconds"`

	// Used to create batches when writing to map store.
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	WriteBatchSize int32 `json:"writeBatchSize,omitempty"`

	// It is meaningful if you are using write behind in MapStore. When it is set to true,
	// only the latest store operation on a key during the write-delay-seconds will be
	// reflected to MapStore.
	// +kubebuilder:default:=true
	// +optional
	WriteCoealescing *bool `json:"writeCoealescing,omitempty"`

	// Properties can be used for giving information to the MapStore implementation
	// +optional
	PropertiesSecretName string `json:"propertiesSecretName,omitempty"`
}

// +kubebuilder:validation:Enum=LAZY;EAGER
type InitialModeType string

const (
	// Loading is asynchronous. It is the default mode.
	InitialModeLazy InitialModeType = "LAZY"
	// Loading is blocked until all partitions are loaded.
	InitialModeEager InitialModeType = "EAGER"
)

type EventJournal struct {
	// Enabled
	// +kubebuilder:default:=false
	Enabled bool `json:"enabled,omitempty"`
	// Capacity
	// +kubebuilder:default:=10000
	Capacity int32 `json:"capacity,omitempty"`
	// TimeToLiveSeconds
	// +kubebuilder:default:=0
	TimeToLiveSeconds int32 `json:"timeToLiveSeconds,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Map is the Schema for the maps API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Map Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Map Config"
type Map struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec MapSpec `json:"spec"`
	// +optional
	Status MapStatus `json:"status,omitempty"`
}

func (m *Map) MapName() string {
	if m.Spec.Name != "" {
		return m.Spec.Name
	}
	return m.Name
}

//+kubebuilder:object:root=true

// MapList contains a list of Map
type MapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Map `json:"items"`
}

func (ml *MapList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(ml.Items))
	for _, item := range ml.Items {
		l = append(l, client.Object(&item))
	}
	return l
}

var (
	EncodeMaxSizePolicy = map[MaxSizePolicyType]int32{
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
		MaxSizePolicyEntryCount:                 10,
	}

	EncodeEvictionPolicyType = map[EvictionPolicyType]int32{
		EvictionPolicyLRU:    0,
		EvictionPolicyLFU:    1,
		EvictionPolicyNone:   2,
		EvictionPolicyRandom: 3,
	}

	EncodeIndexType = map[IndexType]int32{
		IndexTypeSorted: 0,
		IndexTypeHash:   1,
		IndexTypeBitmap: 2,
	}

	EncodeUniqueKeyTransition = map[UniqueKeyTransition]int32{
		UniqueKeyTransitionObject: 0,
		UniqueKeyTransitionLong:   1,
		UniqueKeyTransitionRAW:    2,
	}

	EncodeInMemoryFormat = map[InMemoryFormatType]int32{
		InMemoryFormatBinary: 0,
		InMemoryFormatObject: 1,
		InMemoryFormatNative: 2,
	}
)

func init() {
	SchemeBuilder.Register(&Map{}, &MapList{})
}
