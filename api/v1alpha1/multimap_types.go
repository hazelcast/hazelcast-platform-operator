package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MultiMapSpec defines the desired state of MultiMap
type MultiMapSpec struct {
	DataStructureSpec `json:",inline"`

	// Specifies in which format data will be stored in your multiMap.
	// false: OBJECT true: BINARY
	// +kubebuilder:default:=false
	// +optional
	Binary bool `json:"binary"`

	// Type of the value collection
	// +kubebuilder:default:=SET
	// +optional
	CollectionType CollectionType `json:"collectionType,omitempty"`
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
	DataStructureStatus `json:",inline"`
}

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

func (mm *MultiMap) GetDSName() string {
	if mm.Spec.Name != "" {
		return mm.Spec.Name
	}
	return mm.Name
}

func (mm *MultiMap) GetKind() string {
	return mm.Kind
}

func (mm *MultiMap) GetHZResourceName() string {
	return mm.Spec.HazelcastResourceName
}

func (mm *MultiMap) GetStatus() DataStructureConfigState {
	return mm.Status.State
}

func (mm *MultiMap) GetMemberStatuses() map[string]DataStructureConfigState {
	return mm.Status.MemberStatuses
}

func (mm *MultiMap) SetStatus(status DataStructureConfigState, msg string, memberStatues map[string]DataStructureConfigState) {
	mm.Status.State = status
	mm.Status.Message = msg
	mm.Status.MemberStatuses = memberStatues
}

func (mm *MultiMap) GetSpec() (string, error) {
	mms, err := json.Marshal(mm.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", mm.Kind, err)
	}
	return string(mms), nil
}

func (mm *MultiMap) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &mm.Spec); err != nil {
		return err
	}
	return nil
}

//+kubebuilder:object:root=true

// MultiMapList contains a list of MultiMap
type MultiMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MultiMap `json:"items"`
}

func (mml *MultiMapList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(mml.Items))
	for i := range mml.Items {
		l = append(l, &mml.Items[i])
	}
	return l
}

func init() {
	SchemeBuilder.Register(&MultiMap{}, &MultiMapList{})
}
