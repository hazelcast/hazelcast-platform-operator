package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JetJobSnapshotSpec defines the desired state of JetJobSnapshot
type JetJobSnapshotSpec struct {

	// +required
	Name string `json:"name,omitempty"`

	// +required
	JetJobResourceName string `json:"jetJobResourceName"`

	// ? Adding other data like CreationTime, Size
}

// JetJobSnapshotStatus defines the observed state of JetJobSnapshot
type JetJobSnapshotStatus struct {
	Message string `json:"message,omitempty"`
	// Other fields
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
