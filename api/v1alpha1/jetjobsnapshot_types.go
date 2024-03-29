package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Waiting;Exporting;Exported;Failed;
type JetJobSnapshotState string

const (
	JetJobSnapshotWaiting   JetJobSnapshotState = "Waiting"
	JetJobSnapshotExporting JetJobSnapshotState = "Exporting"
	JetJobSnapshotExported  JetJobSnapshotState = "Exported"
	JetJobSnapshotFailed    JetJobSnapshotState = "Failed"
)

// JetJobSnapshotSpec defines the desired state of JetJobSnapshot
type JetJobSnapshotSpec struct {
	// Name of the exported snapshot
	// +optional
	Name string `json:"name,omitempty"`

	// CancelJob determines whether the job is canceled after exporting snapshot
	// +kubebuilder:default:=false
	// +optional
	CancelJob bool `json:"cancelJob"`

	// JetJobResourceName is the name of the JetJob CR where the Snapshot is exported from
	// +kubebuilder:validation:MinLength:=1
	// +required
	JetJobResourceName string `json:"jetJobResourceName"`
}

// JetJobSnapshotStatus defines the observed state of JetJobSnapshot
type JetJobSnapshotStatus struct {
	// +optional
	State JetJobSnapshotState `json:"state"`

	// +optional
	Message string `json:"message,omitempty"`

	// +optional
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="Current state of the JetJobSnapshot"
// +kubebuilder:printcolumn:name="CreationTime",type="string",JSONPath=".status.creationTime",description="Time when the JetJobSnapshot was created, if created"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the JetJobSnapshot"
// +kubebuilder:resource:shortName=jjs
// JetJobSnapshot is the Schema for the jetjobsnapshots API
type JetJobSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec JetJobSnapshotSpec `json:"spec"`
	// +kubebuilder:default:={state: "Waiting"}
	// +optional
	Status JetJobSnapshotStatus `json:"status,omitempty"`
}

func (jjs *JetJobSnapshot) SnapshotName() string {
	if jjs.Spec.Name != "" {
		return jjs.Spec.Name
	}
	return jjs.Name
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
