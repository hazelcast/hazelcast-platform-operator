package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
)

// MapSpec defines the desired state of Hazelcast Map Config
type MapSpec struct {
	// Name of the map config to be created. If empty, CR name will be used.
	// It cannot be updated after map config is created successfully.
	// +optional
	Name string `json:"name,omitempty"`

	// Count of synchronous backups.
	// It cannot be updated after map config is created successfully.
	// +kubebuilder:default:=1
	// +optional
	BackupCount *int32 `json:"backupCount,omitempty"`

	// Maximum time in seconds for each entry to stay in the map.
	// If it is not 0, entries that are older than this time and not updated for this time are evicted automatically.
	// It can be updated.
	// +kubebuilder:default:=0
	// +optional
	TimeToLiveSeconds *int32 `json:"timeToLiveSeconds,omitempty"`

	// Maximum time in seconds for each entry to stay idle in the map.
	// Entries that are idle for more than this time are evicted automatically.
	// It can be updated.
	// +kubebuilder:default:=0
	// +optional
	MaxIdleSeconds *int32 `json:"maxIdleSeconds,omitempty"`

	// Configuration for removing data from the map when it reaches its max size.
	// It can be updated.
	// +kubebuilder:default:={maxSize: 0}
	// +optional
	Eviction *EvictionConfig `json:"eviction,omitempty"`

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

	// HazelcastResourceName defines the name of the Hazelcast resource.
	// It cannot be updated after map config is created successfully.
	// +kubebuilder:validation:MinLength:=1
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

type EvictionConfig struct {
	// Eviction policy to be applied when map reaches its max size according to the max size policy.
	// +kubebuilder:default:="NONE"
	// +optional
	EvictionPolicy types.EvictionPolicyType `json:"evictionPolicy,omitempty"`

	// Max size of the map.
	// +kubebuilder:default:=0
	// +optional
	MaxSize *int32 `json:"maxSize,omitempty"`

	// Policy for deciding if the maxSize is reached.
	// +kubebuilder:default:="PER_NODE"
	// +optional
	MaxSizePolicy types.MaxSizePolicyType `json:"maxSizePolicy,omitempty"`
}

type IndexConfig struct {
	// Name of the index config.
	// +optional
	Name string `json:"name,omitempty"`

	// Type of the index.
	Type IndexType `json:"type"`

	// Attributes of the index.
	Attributes []string `json:"attributes"`

	// Options for "BITMAP" index type.
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
	UniqueKey string `json:"uniqueKey"`

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
	State          MapConfigState            `json:"state,omitempty"`
	Message        string                    `json:"message,omitempty"`
	MemberStatuses map[string]MapConfigState `json:"memberStatuses,omitempty"`
}

type MapConfigState string

const (
	MapFailed  MapConfigState = "Failed"
	MapSuccess MapConfigState = "Success"
	MapPending MapConfigState = "Pending"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Map is the Schema for the maps API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Map Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Map Config"
type Map struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MapSpec   `json:"spec"`
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

func init() {
	SchemeBuilder.Register(&Map{}, &MapList{})
}

var (
	EncodeInMemoryFormat = map[types.InMemoryFormat]int32{
		types.InMemoryFormatBinary: 0,
		types.InMemoryFormatObject: 1,
		types.InMemoryFormatNative: 2,
	}

	EncodeMaxSizePolicy = map[types.MaxSizePolicyType]int32{
		types.MaxSizePolicyPerNode:                    0,
		types.MaxSizePolicyPerPartition:               1,
		types.MaxSizePolicyUsedHeapPercentage:         2,
		types.MaxSizePolicyUsedHeapSize:               3,
		types.MaxSizePolicyFreeHeapPercentage:         4,
		types.MaxSizePolicyFreeHeapSize:               5,
		types.MaxSizePolicyUsedNativeMemorySize:       6,
		types.MaxSizePolicyUsedNativeMemoryPercentage: 7,
		types.MaxSizePolicyFreeNativeMemorySize:       8,
		types.MaxSizePolicyFreeNativeMemoryPercentage: 9,
	}

	EncodeEvictionPolicyType = map[types.EvictionPolicyType]int32{
		types.EvictionPolicyLRU:    0,
		types.EvictionPolicyLFU:    1,
		types.EvictionPolicyNone:   2,
		types.EvictionPolicyRandom: 3,
	}

	EncodeIndexType = map[IndexType]types.IndexType{
		IndexTypeSorted: types.IndexTypeSorted,
		IndexTypeHash:   types.IndexTypeHash,
		IndexTypeBitmap: types.IndexTypeBitmap,
	}

	EncodeUniqueKeyTransition = map[UniqueKeyTransition]types.UniqueKeyTransformation{
		UniqueKeyTransitionObject: types.UniqueKeyTransformationObject,
		UniqueKeyTransitionLong:   types.UniqueKeyTransformationLong,
		UniqueKeyTransitionRAW:    types.UniqueKeyTransformationRaw,
	}
)
