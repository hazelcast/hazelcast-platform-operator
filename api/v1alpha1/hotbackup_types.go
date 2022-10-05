package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type HotBackupState string

const (
	HotBackupUnknown    HotBackupState = "Unknown"
	HotBackupPending    HotBackupState = "Pending"
	HotBackupNotStarted HotBackupState = "NotStarted"
	HotBackupInProgress HotBackupState = "InProgress"
	HotBackupFailure    HotBackupState = "Failure"
	HotBackupSuccess    HotBackupState = "Success"
)

func (s HotBackupState) IsFinished() bool {
	return s == HotBackupFailure || s == HotBackupSuccess
}

// IsRunning returns true if the HotBackup is scheduled to run or is running but not yet finished.
// Returns false if the HotBackup is not yet scheduled to run or finished it execution including the failure state.
func (s HotBackupState) IsRunning() bool {
	return s == HotBackupInProgress || s == HotBackupPending
}

// HotBackupStatus defines the observed state of HotBackup
type HotBackupStatus struct {
	State   HotBackupState `json:"state"`
	Message string         `json:"message,omitempty"`
}

// HotBackupSpec defines the Spec of HotBackup
type HotBackupSpec struct {
	// HazelcastResourceName defines the name of the Hazelcast resource
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// URL of the bucket to download HotBackup folders.
	// +optional
	BucketURI string `json:"bucketURI"`

	// Name of the secret with credentials for cloud providers.
	// +optional
	Secret string `json:"secret"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HotBackup is the Schema for the hot backup API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the HotBackup process"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current HotBackup Config"
// +kubebuilder:resource:shortName=hb
type HotBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Status HotBackupStatus `json:"status,omitempty"`
	Spec   HotBackupSpec   `json:"spec"`
}

//+kubebuilder:object:root=true

// HotBackupList contains a list of HotBackup
type HotBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HotBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HotBackup{}, &HotBackupList{})
}
