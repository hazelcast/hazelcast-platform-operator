package v1alpha1

import (
	"fmt"

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

	// +required
	Port int32 `json:"port,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource that this resource is
	// created for.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`
}

// HazelcastEndpointStatus defines the observed state of HazelcastEndpoint
type HazelcastEndpointStatus struct {
	// Address of the HazelcastEndpoint
	// +optional
	Address string `json:"address,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HazelcastEndpoint is the Schema for the hazelcastendpoints API
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="Type of the HazelcastEndpoint"
// +kubebuilder:printcolumn:name="Address",type="string",JSONPath=".status.address",description="Address of the HazelcastEndpoint"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current HazelcastEndpoint"
// +kubebuilder:resource:shortName=hzep
type HazelcastEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HazelcastEndpointSpec   `json:"spec,omitempty"`
	Status HazelcastEndpointStatus `json:"status,omitempty"`
}

func (hzEndpoint *HazelcastEndpoint) SetAddress(address string) {
	if address == "" {
		hzEndpoint.Status.Address = ""
	} else {
		hzEndpoint.Status.Address = fmt.Sprintf("%s:%d", address, hzEndpoint.Spec.Port)
	}
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
