package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MapSpec defines the desired state of Map
type MapSpec struct {
	Name               string          `json:"name,omitempty"`
	BackupCount        int             `json:"backupCount,omitempty"`
	AsyncBackupCount   int             `json:"asyncBackupCount,omitempty"`
	TimeToLiveSeconds  int             `json:"timeToLiveSeconds,omitempty"`
	MaxIdleSeconds     int             `json:"maxIdleSeconds,omitempty"`
	Eviction           *EvictionConfig `json:"eviction,omitempty"`
	ReadBackupData     string          `json:"readBackupData,omitempty"`
	Indexes            []IndexConfig   `json:"indexes,omitempty"`
	PersistenceEnabled string          `json:"persistenceEnabled,omitempty"`
}

type EvictionConfig struct {
	EvictionPolicy string `json:"persistenceEnabled,omitempty"`
	MaxSize        string `json:"maxSize,omitempty"`
	MaxSizePolicy  string `json:"maxSizePolicy,omitempty"`
}

type IndexConfig struct {
	Name               string                    `json:"name,omitempty"`
	Type               string                    `json:"type,omitempty"`
	Attributes         string                    `json:"attributes,omitempty"`
	BitMapIndexOptions *BitMapIndexOptionsConfig `json:"bitMapIndexOptions,omitempty"`
}

type BitMapIndexOptionsConfig struct {
	UniqueKey           string `json:"uniqueKey,omitempty"`
	UniqueKeyTransition string `json:"uniqueKeyTransition,omitempty"`
}

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
