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
	AsyncFillup bool `json:"asyncFillup,omitempty"`

	// InMemoryFormat specifies in which format data will be stored in the ReplicatedMap
	// +kubebuilder:default:=OBJECT
	// +optional
	InMemoryFormat InMemoryFormatType `json:"inMemoryFormat,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource.
	// +kubebuilder:validation:MinLength:=1
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// ReplicatedMapStatus defines the observed state of ReplicatedMap
type ReplicatedMapStatus struct {
	DataStructureStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ReplicatedMap is the Schema for the replicatedmaps API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the ReplicatedMap Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current ReplicatedMap Config"
// +kubebuilder:resource:shortName=rmap
type ReplicatedMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicatedMapSpec   `json:"spec,omitempty"`
	Status ReplicatedMapStatus `json:"status,omitempty"`
}

func (mm *ReplicatedMap) GetDSName() string {
	if mm.Spec.Name != "" {
		return mm.Spec.Name
	}
	return mm.Name
}

func (mm *ReplicatedMap) GetKind() string {
	return mm.Kind
}

func (mm *ReplicatedMap) GetHZResourceName() string {
	return mm.Spec.HazelcastResourceName
}

func (mm *ReplicatedMap) GetStatus() DataStructureConfigState {
	return mm.Status.State
}

func (mm *ReplicatedMap) GetMemberStatuses() map[string]DataStructureConfigState {
	return mm.Status.MemberStatuses
}

func (mm *ReplicatedMap) SetStatus(status DataStructureConfigState, msg string, memberStatues map[string]DataStructureConfigState) {
	mm.Status.State = status
	mm.Status.Message = msg
	mm.Status.MemberStatuses = memberStatues
}

func (mm *ReplicatedMap) GetSpec() (string, error) {
	mms, err := json.Marshal(mm.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", mm.Kind, err)
	}
	return string(mms), nil
}

func (mm *ReplicatedMap) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &mm.Spec); err != nil {
		return err
	}
	return nil
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
	for _, item := range rml.Items {
		l = append(l, client.Object(&item))
	}
	return l
}

func init() {
	SchemeBuilder.Register(&ReplicatedMap{}, &ReplicatedMapList{})
}
