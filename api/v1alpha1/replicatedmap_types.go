package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReplicatedMapSpec defines the desired state of ReplicatedMap
type ReplicatedMapSpec struct {
	// Name of the ReplicatedMap config to be created. If empty, CR name will be used.
	// +optional
	Name string `json:"name,omitempty"`

	// AsyncFillup specifies whether the ReplicatedMap is available for reads before the initial replication is completed
	// +kubebuilder:default:=true
	// +optional
	AsyncFillup *bool `json:"asyncFillup,omitempty"`

	// InMemoryFormat specifies in which format data will be stored in the ReplicatedMap
	// +kubebuilder:default:=OBJECT
	// +optional
	InMemoryFormat RMInMemoryFormatType `json:"inMemoryFormat,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// Name of the User Code Namespace applied to this instance
	// +optional
	UserCodeNamespace string `json:"userCodeNamespace,omitempty"`
}

// ReplicatedMapStatus defines the observed state of ReplicatedMap
type ReplicatedMapStatus struct {
	DataStructureStatus `json:",inline"`
}

// RMInMemoryFormatType represents the format options for storing the data in the ReplicatedMap.
// +kubebuilder:validation:Enum=BINARY;OBJECT
type RMInMemoryFormatType string

const (
	// RMInMemoryFormatBinary Data will be stored in serialized binary format.
	RMInMemoryFormatBinary RMInMemoryFormatType = "BINARY"

	// RMInMemoryFormatObject Data will be stored in deserialized form.
	RMInMemoryFormatObject RMInMemoryFormatType = "OBJECT"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ReplicatedMap is the Schema for the replicatedmaps API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the ReplicatedMap Config"
// +kubebuilder:printcolumn:name="Hazelcast-Resource",type="string",priority=1,JSONPath=".spec.hazelcastResourceName",description="Name of the Hazelcast resource that this resource is created for"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current ReplicatedMap Config"
// +kubebuilder:resource:shortName=rmap
type ReplicatedMap struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec ReplicatedMapSpec `json:"spec"`
	// +optional
	Status ReplicatedMapStatus `json:"status,omitempty"`
}

func (rm *ReplicatedMap) GetDSName() string {
	if rm.Spec.Name != "" {
		return rm.Spec.Name
	}
	return rm.Name
}

func (rm *ReplicatedMap) GetKind() string {
	return rm.Kind
}

func (rm *ReplicatedMap) GetHZResourceName() string {
	return rm.Spec.HazelcastResourceName
}

func (rm *ReplicatedMap) GetStatus() *DataStructureStatus {
	return &rm.Status.DataStructureStatus
}

func (rm *ReplicatedMap) GetSpec() (string, error) {
	rms, err := json.Marshal(rm.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", rm.Kind, err)
	}
	return string(rms), nil
}

func (rm *ReplicatedMap) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &rm.Spec); err != nil {
		return err
	}
	return nil
}

func (rm *ReplicatedMap) ValidateSpecCurrent(_ *Hazelcast) error {
	return nil
}

func (rm *ReplicatedMap) ValidateSpecUpdate() error {
	return validateReplicatedMapSpecUpdate(rm)
}

//+kubebuilder:object:root=true

// ReplicatedMapList contains a list of ReplicatedMap
type ReplicatedMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicatedMap `json:"items"`
}

func (rml *ReplicatedMapList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(rml.Items))
	for i := range rml.Items {
		l = append(l, client.Object(&rml.Items[i]))
	}
	return l
}

func init() {
	SchemeBuilder.Register(&ReplicatedMap{}, &ReplicatedMapList{})
}
