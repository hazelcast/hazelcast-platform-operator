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

	// globalOrderingEnabled allows all nodes listening to the same topic get their messages in the same order
	// the same order
	// +kubebuilder:default:=false
	// +optional
	GlobalOrderingEnabled bool `json:"globalOrderingEnabled"`

	// multiThreadingEnabled enables multi-threaded processing of incoming messages
	// a single thread will handle all topic messages
	// +kubebuilder:default:=false
	// +optional
	MultiThreadingEnabled bool `json:"multiThreadingEnabled"`

	// HazelcastResourceName defines the name of the Hazelcast resource for which
	// topic config will be created
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// Name of the User Code Namespace applied to this instance
	// +kubebuilder:validation:MinLength:=1
	// +optional
	UserCodeNamespace string `json:"userCodeNamespace,omitempty"`
}

// TopicStatus defines the observed state of Topic
type TopicStatus struct {
	DataStructureStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Topic Config"
// +kubebuilder:printcolumn:name="Hazelcast-Resource",type="string",priority=1,JSONPath=".spec.hazelcastResourceName",description="Name of the Hazelcast resource that this resource is created for"
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

func (t *Topic) GetDSName() string {
	if t.Spec.Name != "" {
		return t.Spec.Name
	}
	return t.Name
}

func (t *Topic) GetKind() string {
	return t.Kind
}

func (t *Topic) GetHZResourceName() string {
	return t.Spec.HazelcastResourceName
}

func (t *Topic) GetStatus() *DataStructureStatus {
	return &t.Status.DataStructureStatus
}

func (t *Topic) GetSpec() (string, error) {
	ts, err := json.Marshal(t.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", t.Kind, err)
	}
	return string(ts), nil
}

func (t *Topic) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &t.Spec); err != nil {
		return err
	}
	return nil
}

func (t *Topic) ValidateSpecCurrent(_ *Hazelcast) error {
	return validateTopicSpecCurrent(t)
}

func (t *Topic) ValidateSpecUpdate() error {
	return validateTopicSpecUpdate(t)
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
