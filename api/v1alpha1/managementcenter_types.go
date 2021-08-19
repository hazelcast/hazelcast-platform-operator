package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManagementCenterSpec defines the desired state of ManagementCenter
type ManagementCenterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ManagementCenter. Edit managementcenter_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// ManagementCenterStatus defines the observed state of ManagementCenter
type ManagementCenterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ManagementCenter is the Schema for the managementcenters API
type ManagementCenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagementCenterSpec   `json:"spec,omitempty"`
	Status ManagementCenterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManagementCenterList contains a list of ManagementCenter
type ManagementCenterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagementCenter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagementCenter{}, &ManagementCenterList{})
}
