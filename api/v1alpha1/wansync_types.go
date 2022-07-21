package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WanSyncSpec defines the desired state of WanSync
type WanSyncSpec struct {
	// WanSource represents the source of a WAN publisher.
	WanSource `json:",inline"`
}

// WanSource represents the source of a WAN publisher.
// Only one of its members may be specified.
type WanSource struct {
	// WanReplicationName is the name of WanReplication CR that contains the WAN publisher configuration.
	// If specified the Sync operation will use existing WAN publisher.
	// +optional
	WanReplicationName string `json:"wanReplicationName,omitempty"`

	// Config is the configuration for the WAN replication publisher.
	// If specified the Syc operation will create a new WAN publisher.
	// +optional
	Config *WanSyncConfig `json:"config,omitempty"`
}

type WanSyncConfig struct {
	// MapResourceName is the name of Map custom resource which WAN replication will be applied to.
	// +kubebuilder:validation:MinLength:=1
	MapResourceName string `json:"mapResourceName"`

	// TargetClusterName is the clusterName field of the target Hazelcast resource.
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

	// Sync is the configuration for the WAN sync mechanism.
	// +optional
	Sync *Sync `json:"sync,omitempty"`
}

// WanSyncStatus defines the observed state of WanSync
type WanSyncStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WanSync is the Schema for the wansyncs API
type WanSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WanSyncSpec   `json:"spec,omitempty"`
	Status WanSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WanSyncList contains a list of WanSync
type WanSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WanSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WanSync{}, &WanSyncList{})
}
