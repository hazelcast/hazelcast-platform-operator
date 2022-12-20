package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TopicSpec defines the desired state of Topic
type TopicSpec struct {
	// Name of the topic config to be created. If empty, CR name will be used.
	// +optional
	Name string `json:"name,omitempty"`

	// When true all nodes listening to the same topic get their messages in
	// the same order
	// +kubebuilder:default:=false
	// +optional
	GlobalOrderingEnabled bool `json:"globalOrderingEnabled"`

	// When true enables multi-threaded processing of incoming messages, otherwise
	// a single thread will handle all topic messages
	// +kubebuilder:default:=false
	// +optional
	MultiThreadingEnabled bool `json:"multiThreadingEnabled"`

	// HazelcastResourceName defines the name of the Hazelcast resource for which
	// topic config will be created
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// TopicStatus defines the observed state of Topic
type TopicStatus struct {
	DataStructureStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Topic Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Topic Config"

// Topic is the Schema for the topics API
type Topic struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec TopicSpec `json:"spec"`
	// +optional
	Status TopicStatus `json:"status,omitempty"`
}

func (mm *Topic) GetDSName() string {
	if mm.Spec.Name != "" {
		return mm.Spec.Name
	}
	return mm.Name
}

func (mm *Topic) GetKind() string {
	return mm.Kind
}

func (mm *Topic) GetHZResourceName() string {
	return mm.Spec.HazelcastResourceName
}

func (mm *Topic) GetStatus() DataStructureConfigState {
	return mm.Status.State
}

func (mm *Topic) GetMemberStatuses() map[string]DataStructureConfigState {
	return mm.Status.MemberStatuses
}

func (mm *Topic) SetStatus(status DataStructureConfigState, msg string, memberStatues map[string]DataStructureConfigState) {
	mm.Status.State = status
	mm.Status.Message = msg
	mm.Status.MemberStatuses = memberStatues
}

func (mm *Topic) GetSpec() (string, error) {
	mms, err := json.Marshal(mm.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", mm.Kind, err)
	}
	return string(mms), nil
}

func (mm *Topic) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &mm.Spec); err != nil {
		return err
	}
	return nil
}

//+kubebuilder:object:root=true

// TopicList contains a list of Topic
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Topic `json:"items"`
}

func (tl *TopicList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(tl.Items))
	for i := range tl.Items {
		l = append(l, &tl.Items[i])
	}
	return l
}

func init() {
	SchemeBuilder.Register(&Topic{}, &TopicList{})
}
