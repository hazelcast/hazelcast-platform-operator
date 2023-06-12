package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Waiting;Starting;Running;Suspended;SuspendedExportingSnapshot;Completing;Failed;Completed
type JetJobSnapshotState string

const (
	JetJobSnapshotWaiting   JetJobSnapshotState = "Waiting"
	JetJobSnapshotExporting JetJobSnapshotState = "Exporting"
	JetJobSnapshotExported  JetJobSnapshotState = "Exported"
	JetJobSnapshotFailed    JetJobSnapshotState = "Failed"
)

// JetJobSnapshotSpec defines the desired state of JetJobSnapshot
type JetJobSnapshotSpec struct {
	// +required
	Name string `json:"name"`

	// +kubebuilder:default:=false
	// +optional
	CancelJob bool `json:"cancelJob"`

	// +required
	JetJobResourceName string `json:"jetJobResourceName"`
}

// JetJobSnapshotStatus defines the observed state of JetJobSnapshot
type JetJobSnapshotStatus struct {
	// +optional
	State JetJobSnapshotState `json:"state,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// creation time

	// size
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// JetJobSnapshot is the Schema for the jetjobsnapshots API
type JetJobSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JetJobSnapshotSpec   `json:"spec,omitempty"`
	Status JetJobSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// JetJobSnapshotList contains a list of JetJobSnapshot
type JetJobSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JetJobSnapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JetJobSnapshot{}, &JetJobSnapshotList{})
}
