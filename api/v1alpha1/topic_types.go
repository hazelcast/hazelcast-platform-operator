package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// TopicStatus defines the observed state of Topic
type TopicStatus struct {
	State          TopicConfigState            `json:"state,omitempty"`
	Message        string                      `json:"message,omitempty"`
	MemberStatuses map[string]TopicConfigState `json:"memberStatuses,omitempty"`
}

// +kubebuilder:validation:Enum=Success;Failed;Pending;Persisting;Terminating
type TopicConfigState string

const (
	TopicFailed  TopicConfigState = "Failed"
	TopicSuccess TopicConfigState = "Success"
	TopicPending TopicConfigState = "Pending"
	// Topic config is added into all members but waiting for Topic to be persisten into ConfigTopic
	TopicPersisting  TopicConfigState = "Persisting"
	TopicTerminating TopicConfigState = "Terminating"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Topic Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Topic Config"

// Topic is the Schema for the topics API
type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopicSpec   `json:"spec"`
	Status TopicStatus `json:"status,omitempty"`
}

func (mm *Topic) TopicName() string {
	if mm.Spec.Name != "" {
		return mm.Spec.Name
	}
	return mm.Name
}

//+kubebuilder:object:root=true

// TopicList contains a list of Topic
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Topic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Topic{}, &TopicList{})
}
