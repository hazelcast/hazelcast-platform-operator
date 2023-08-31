package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Discovery;Member;WAN;
type HazelcastEndpointType string

const (
	HazelcastEndpointTypeDiscovery HazelcastEndpointType = "Discovery"
	HazelcastEndpointTypeMember    HazelcastEndpointType = "Member"
	HazelcastEndpointTypeWAN       HazelcastEndpointType = "WAN"
)

// HazelcastEndpointSpec defines the desired state of HazelcastEndpoint
type HazelcastEndpointSpec struct {
	// +required
	Type HazelcastEndpointType `json:"type"`

	// +optional
	Address string `json:"foo,omitempty"`

	// +optional
	Port int32 `json:"port,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource that this resource is
	// created for.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// HazelcastEndpointStatus defines the observed state of HazelcastEndpoint
type HazelcastEndpointStatus struct {
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HazelcastEndpoint is the Schema for the hazelcastendpoints API
type HazelcastEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HazelcastEndpointSpec   `json:"spec,omitempty"`
	Status HazelcastEndpointStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HazelcastEndpointList contains a list of HazelcastEndpoint
type HazelcastEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HazelcastEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HazelcastEndpoint{}, &HazelcastEndpointList{})
}
