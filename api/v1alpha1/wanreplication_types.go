package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WanReplicationSpec defines the desired state of WanReplication
type WanReplicationSpec struct {
	// MapResourceName is the name of Map custom resource which WAN replication will be applied to.
	// +kubebuilder:validation:MinLength:=1
	MapResourceName string `json:"mapResourceName"`

	// ClusterName is the clusterName field of the target Hazelcast resource.
	// +kubebuilder:validation:MinLength:=1
	TargetClusterName string `json:"targetClusterName"`

	// Endpoints is the target cluster endpoints.
	// +kubebuilder:validation:MinLength:=1
	Endpoints string `json:"endpoints"`

	// Queue is the configuration for WAN events queue.
	// +optional
	Queue QueueSetting `json:"queue,omitempty"`

	// Batch is the configuration for WAN events batch.
	// +optional
	Batch BatchSetting `json:"batch,omitempty"`

	// Acknowledgement is the configuration for the condition when the next batch of WAN events are sent.
	// +optional
	Acknowledgement AcknowledgementSetting `json:"acknowledgement,omitempty"`
}

// QueueSetting defines the configuration for Hazelcast WAN queue
type QueueSetting struct {
	// Capacity is the total capacity of WAN queue.
	// +kubebuilder:default:=10000
	// +optional
	Capacity int32 `json:"capacity,omitempty"`

	// FullBehavior represents the behavior of the new arrival when the queue is full.
	// +kubebuilder:validation:Enum=DISCARD_AFTER_MUTATION;THROW_EXCEPTION;THROW_EXCEPTION_ONLY_IF_REPLICATION_ACTIVE
	// +kubebuilder:default:=DISCARD_AFTER_MUTATION
	// +optional
	FullBehavior FullBehaviorSetting `json:"fullBehavior,omitempty"`
}

type FullBehaviorSetting string

const (
	DiscardAfterMutation FullBehaviorSetting = "DISCARD_AFTER_MUTATION"

	ThrowException FullBehaviorSetting = "THROW_EXCEPTION"

	ThrowExceptionOnlyIfReplicationActive = "THROW_EXCEPTION_ONLY_IF_REPLICATION_ACTIVE"
)

type BatchSetting struct {
	// Size represents the maximum batch size.
	// +kubebuilder:default:=500
	// +optional
	Size int32 `json:"size,omitempty"`

	// MaximumDelay represents the maximum delay in milliseconds.
	// If the batch size is not reached, the events will be sent after
	// the maximum delay.
	// +kubebuilder:default:=1000
	// +optional
	MaximumDelay int32 `json:"maximumDelay,omitempty"`
}

type AcknowledgementSetting struct {
	// Type represents how a batch of replication events is considered successfully replicated.
	// +kubebuilder:validation:Enum=ACK_ON_OPERATION_COMPLETE;ACK_ON_RECEIPT
	// +kubebuilder:default:=ACK_ON_OPERATION_COMPLETE
	// +optional
	Type AcknowledgementType `json:"type,omitempty"`

	// Timeout represents the time the source cluster waits for the acknowledgement.
	// After timeout, the events will be resent.
	// +kubebuilder:default:=60000
	// +optional
	Timeout int32 `json:"timeout,omitempty"`
}

type AcknowledgementType string

const (
	AckOnReceipt AcknowledgementType = "ACK_ON_RECEIPT"

	AckOnOperationComplete AcknowledgementType = "ACK_ON_OPERATION_COMPLETE"
)

type WanStatus string

const (
	WanStatusFailed  WanStatus = "Failed"
	WanStatusPending WanStatus = "Pending"
	WanStatusSuccess WanStatus = "Success"
)

// WanReplicationStatus defines the observed state of WanReplication
type WanReplicationStatus struct {
	// PublisherId is the ID used for WAN publisher ID
	PublisherId string `json:"publisherId,omitempty"`

	// Status is the status of WAN replication
	Status WanStatus `json:"status,omitempty"`

	// Message is the field to show detail information or error
	Message string `json:"message,omitempty"`
}

// WanReplication is the Schema for the wanreplications API
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status",description="Current state of the Hazelcast WAN Replication"
//+kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current WanReplication Config"
//+kubebuilder:resource:shortName=wr
type WanReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WanReplicationSpec   `json:"spec,omitempty"`
	Status WanReplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WanReplicationList contains a list of WanReplication
type WanReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WanReplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WanReplication{}, &WanReplicationList{})
}
