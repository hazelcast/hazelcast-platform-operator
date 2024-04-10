package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserCodeNamespaceSpec defines the desired state of UserCodeNamespace
type UserCodeNamespaceSpec struct {
	// Bucket config from where the JAR files will be downloaded.
	// +optional
	BucketConfiguration *BucketConfiguration `json:"bucketConfig,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource that this resource is
	// created for.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// +kubebuilder:validation:Enum=Unknown;Pending;NotStarted;InProgress;Failure;Success
type UserCodeNamespaceState string

const (
	UserCodeNamespacePending UserCodeNamespaceState = "Pending"
	UserCodeNamespaceFailure UserCodeNamespaceState = "Failure"
	UserCodeNamespaceSuccess UserCodeNamespaceState = "Success"
)

// UserCodeNamespaceStatus defines the observed state of UserCodeNamespace
type UserCodeNamespaceStatus struct {
	// +optional
	State UserCodeNamespaceState `json:"state,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Status of the User Code Namespace"

// UserCodeNamespace is the Schema for the usercodenamespaces API
type UserCodeNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserCodeNamespaceSpec   `json:"spec,omitempty"`
	Status UserCodeNamespaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserCodeNamespaceList contains a list of UserCodeNamespace
type UserCodeNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserCodeNamespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserCodeNamespace{}, &UserCodeNamespaceList{})
}
