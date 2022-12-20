package v1alpha1

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QueueSpec defines the desired state of Queue
// It cannot be updated after the Queue is created
type QueueSpec struct {
	DataStructureSpec `json:",inline"`

	// Max size of the queue.
	// +kubebuilder:default:=0
	// +optional
	MaxSize *int32 `json:"maxSize,omitempty"`

	// Time in seconds after which the Queue will be destroyed if it stays empty or unused.
	// If the values is not provided the Queue will never be destroyed.
	// +kubebuilder:default:=-1
	// +optional
	EmptyQueueTtlSeconds *int32 `json:"emptyQueueTTLSeconds,omitempty"`

	// The name of the comparator class.
	// If the class name is provided, the Queue becomes Priority Queue.
	// You can learn more at https://docs.hazelcast.com/hazelcast/latest/data-structures/priority-queue.
	// +optional
	PriorityComparatorClassName string `json:"priorityComparatorClassName,omitempty"`
}

// QueueStatus defines the observed state of Queue
type QueueStatus struct {
	DataStructureStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the Queue Config"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Queue Config"
// +kubebuilder:resource:shortName=q

// Queue is the Schema for the queues API
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec,omitempty"`
	Status QueueStatus `json:"status,omitempty"`
}

func (q *Queue) GetKind() string {
	return q.Kind
}

func (q *Queue) GetDSName() string {
	if q.Spec.Name != "" {
		return q.Spec.Name
	}
	return q.Name
}

func (q *Queue) GetHZResourceName() string {
	return q.Spec.HazelcastResourceName
}

func (q *Queue) GetStatus() DataStructureConfigState {
	return q.Status.State
}

func (q *Queue) GetMemberStatuses() map[string]DataStructureConfigState {
	return q.Status.MemberStatuses
}

func (q *Queue) SetStatus(state DataStructureConfigState, msg string, memberStatues map[string]DataStructureConfigState) {
	q.Status.State = state
	q.Status.Message = msg
	q.Status.MemberStatuses = memberStatues
}

func (q *Queue) GetSpec() (string, error) {
	qs, err := json.Marshal(q.Spec)
	if err != nil {
		return "", fmt.Errorf("error marshaling %v as JSON: %w", q.Kind, err)
	}
	return string(qs), nil
}

func (q *Queue) SetSpec(spec string) error {
	if err := json.Unmarshal([]byte(spec), &q.Spec); err != nil {
		return err
	}
	return nil
}

//+kubebuilder:object:root=true

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

func (ql *QueueList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(ql.Items))
	for i := range ql.Items {
		l = append(l, &ql.Items[i])
	}
	return l
}

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
