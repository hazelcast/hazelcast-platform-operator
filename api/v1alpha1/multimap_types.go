package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MultiMapSpec defines the desired state of MultiMap
type MultiMapSpec struct {
	// Name of the multiMap config to be created. If empty, CR name will be used.
	// +optional
	Name string `json:"name,omitempty"`

	// Count of synchronous backups.
	// +kubebuilder:default:=1
	// +optional
	BackupCount *int32 `json:"backupCount,omitempty"`

	// Specifies in which format data will be stored in your multiMap.
	// false: OBJECT true: BINARY
	// +kubebuilder:default:=false
	// +optional
	Binary bool `json:"binary"`

	// Type of the value collection
	// +kubebuilder:default:=SET
	// +optional
	CollectionType CollectionType `json:"collectionType,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource.
	// +kubebuilder:validation:MinLength:=1
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// CollectionType represents the value collection options for storing the data in the multiMap.
// +kubebuilder:validation:Enum=SET;LIST
type CollectionType string

const (
	CollectionTypeSet CollectionType = "SET"

	CollectionTypeList CollectionType = "LIST"
)

// MultiMapStatus defines the observed state of MultiMap
type MultiMapStatus struct {
	State          MultiMapConfigState            `json:"state,omitempty"`
	Message        string                         `json:"message,omitempty"`
	MemberStatuses map[string]MultiMapConfigState `json:"memberStatuses,omitempty"`
}

// +kubebuilder:validation:Enum=Success;Failed;Pending;Persisting;Terminating
type MultiMapConfigState string

const (
	MultiMapFailed  MultiMapConfigState = "Failed"
	MultiMapSuccess MultiMapConfigState = "Success"
	MultiMapPending MultiMapConfigState = "Pending"
	// MultiMap config is added into all members but waiting for multiMap to be persisten into ConfigMap
	MultiMapPersisting  MultiMapConfigState = "Persisting"
	MultiMapTerminating MultiMapConfigState = "Terminating"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MultiMap is the Schema for the multimaps API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the MultiMap Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current MultiMap Config"
// +kubebuilder:resource:shortName=mmap
type MultiMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiMapSpec   `json:"spec,omitempty"`
	Status MultiMapStatus `json:"status,omitempty"`
}

func (mm *MultiMap) MultiMapName() string {
	if mm.Spec.Name != "" {
		return mm.Spec.Name
	}
	return mm.Name
}

//+kubebuilder:object:root=true

// MultiMapList contains a list of MultiMap
type MultiMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiMap `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiMap{}, &MultiMapList{})
}
