package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MapSpec defines the desired state of Map
type MapSpec struct {
	// Name of the map config to be created. If empty, CR name will be used.
	// +optional
	Name string `json:"name,omitempty"`

	// Count of synchronous backups.
	// +kubebuilder:default:=1
	// +optional
	BackupCount *int32 `json:"backupCount,omitempty"`

	// Number of asynchronous backups. Unlike the synchronous backup process,
	// asynchronous backup process does not block the map operations.
	// +kubebuilder:default:=0
	// +optional
	AsyncBackupCount *int32 `json:"asyncBackupCount,omitempty"`

	// Maximum time in seconds for each entry to stay in the map.
	// If it is not 0, entries that are older than this time and not updated for this time are evicted automatically
	// +kubebuilder:default:=0
	// +optional
	TimeToLiveSeconds *int32 `json:"timeToLiveSeconds,omitempty"`

	// Maximum time in seconds for each entry to stay idle in the map.
	// Entries that are idle for more than this time are evicted automatically.
	// +kubebuilder:default:=0
	// +optional
	MaxIdleSeconds *int32 `json:"maxIdleSeconds,omitempty"`

	// Configuration for removing data from the map when it reaches its max size.
	// +optional
	Eviction *EvictionConfig `json:"eviction,omitempty"`

	// Used to enable reading from local backup map entries.
	// It can be used if there is at least 1 sync or async backup.
	// +kubebuilder:default:=false
	// +optional
	ReadBackupData bool `json:"readBackupData,omitempty"`

	// Indexes to be created for the map data.
	// You can learn more at https://docs.hazelcast.com/hazelcast/latest/query/indexing-maps
	// +optional
	Indexes []IndexConfig `json:"indexes,omitempty"`

	// When enabled, map data will be persisted
	// +kubebuilder:default:=false
	// +optional
	PersistenceEnabled bool `json:"persistenceEnabled,omitempty"`
}

type EvictionConfig struct {
	// +kubebuilder:default:="NONE"
	// +optional
	EvictionPolicy string `json:"persistenceEnabled,omitempty"`

	// +kubebuilder:default:=0
	// +optional
	MaxSize *int32 `json:"maxSize,omitempty"`

	// +kubebuilder:default:="PER_NODE"
	// +optional
	MaxSizePolicy string `json:"maxSizePolicy,omitempty"`
}

type IndexConfig struct {
	// Name of the index config, optional.
	// +optional
	Name string `json:"name,omitempty"`

	Type IndexType `json:"type"`

	Attributes []string `json:"attributes"`

	// +optional
	BitMapIndexOptions *BitMapIndexOptionsConfig `json:"bitMapIndexOptions,omitempty"`
}

// +kubebuilder:validation:Enum=SORTED;HASH;BITMAP
type IndexType string

const (
	IndexTypeSorted IndexType = "SORTED"
	IndexTypeHash   IndexType = "HASH"
	IndexTypeBitmap IndexType = "BITMAP"
)

type BitMapIndexOptionsConfig struct {
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
	State MapConfigState `json:"state,omitempty"`
}

type MapConfigState string

const (
	MapFailed  MapConfigState = "Failed"
	MapSuccess MapConfigState = "Success"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Map is the Schema for the maps API
type Map struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MapSpec   `json:"spec,omitempty"`
	Status MapStatus `json:"status,omitempty"`
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
