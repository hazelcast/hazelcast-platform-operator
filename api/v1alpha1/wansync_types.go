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
// +kubebuilder:validation:MinProperties:=1
// +kubebuilder:validation:MaxProperties:=1
type WanSource struct {
	// WanReplicationName is the name of WanReplication CR that contains the WAN publisher configuration.
	// If specified the Sync operation will use existing WAN publisher.
	// +optional
	WanReplicationName string `json:"wanReplicationName,omitempty"`

	// Config is the configuration for the WAN replication publisher.
	// If specified the Syc operation will create a new WAN publisher.
	// +optional
	Config *WanPublisherConfig `json:"config,omitempty"`
}

type WanSyncPhase string

const (
	WanSyncNotStarted WanSyncPhase = "NotStarted"
	WanSyncFailed     WanSyncPhase = "Failed"
	WanSyncPending    WanSyncPhase = "Pending"
	WanSyncCompleted  WanSyncPhase = "Completed"
)

// WanSyncStatus defines the observed state of WanSync
type WanSyncStatus struct {

	// Status is the status of WAN Sync
	// +optional
	Status WanSyncPhase `json:"status,omitempty"`

	// Message is the field to show detail information or error
	// +optional
	Message string `json:"message,omitempty"`

	// WanSyncMapStatus is the WAN Sync status of the Maps given in the spec
	// directly or indirectly by Hazelcast resource.
	// +optional
	WanSyncMapsStatus map[string]WanSyncMapStatus `json:"wanSyncMapsStatus,omitempty"`
}

type WanSyncMapStatus struct {
	// ResourceName is the name of the Map Custom Resource.
	// +optional
	ResourceName string `json:"resourceName,omitempty"`

	// PublisherId is the ID used for WAN publisher ID
	PublisherId string `json:"publisherId,omitempty"`

	// Status is the status of the resource WAN sync
	Phase WanSyncPhase `json:"phase,omitempty"`

	// Message is the field to show detail information or error
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status",description="Current state of the Hazelcast WAN Sync"

// WanSync is the Schema for the wansyncs API
type WanSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WanSyncSpec   `json:"spec,omitempty"`
	Status WanSyncStatus `json:"status,omitempty"`
}

func (w *WanSync) WanPublisherConfig() *WanPublisherConfig {
	return w.Spec.Config
}

func (w *WanSync) PublisherId(mapName string) string {
	return w.Status.WanSyncMapsStatus[mapName].PublisherId
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
