package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HotBackupStatus defines the observed state of HotBackup
type HotBackupStatus struct {
}

// HotBackupSpec defines the Spec of HotBackup
type HotBackupSpec struct {
	// HazelcastResourceName defines the name of the Hazelcast resource
	// +kubebuilder:validation:Required
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HotBackup is the Schema for the hot backup API
type HotBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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
