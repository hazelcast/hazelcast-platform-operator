package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HazelcastSpec defines the desired state of Hazelcast
type HazelcastSpec struct {
	// Number of Hazelcast members in the cluster.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=3
	// +optional
	ClusterSize int32 `json:"clusterSize"`

	// Repository to pull the Hazelcast Platform image from.
	// +kubebuilder:default:="docker.io/hazelcast/hazelcast-enterprise"
	// +optional
	Repository string `json:"repository"`

	// Version of Hazelcast Platform.
	// +kubebuilder:default:="5.0-SNAPSHOT"
	// +optional
	Version string `json:"version"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +kubebuilder:default:="hazelcast-license-key"
	// +optional
	LicenseKeySecret string `json:"licenseKeySecret"`

	// Configuration to expose Hazelcast cluster to external clients.
	// +optional
	ExposeExternally ExposeExternallyConfiguration `json:"exposeExternally"`
}

func (c ExposeExternallyConfiguration) IsEnabled() bool {
	return !(c == (ExposeExternallyConfiguration{}))
}

// ExposeExternallyConfiguration defines how to expose Hazelcast cluster to external clients
type ExposeExternallyConfiguration struct {
	// Specifies how members are exposed.
	// Valid values are:
	// - "Smart" (default): each member pod is exposed with a separate external address
	// - "Unisocket": all member pods are exposed with one external address
	// +optional
	Type ExposeExternallyType `json:"type,omitempty"`

	// Type of the service used to discover Hazelcast cluster.
	// +optional
	DiscoveryServiceType corev1.ServiceType `json:"discoveryServiceType,omitempty"`

	// Method of how each member is accessed from the external client.
	// Valid values are:
	// - "NodeExternalIP" (default): each member is accessed by the NodePort service and the node external IP/hostname
	// - "NodeName": each member is accessed by the NodePort service and the node name
	// - "LoadBalancer": each member is accessed by the LoadBalancer service external address
	// +optional
	MemberAccess MemberAccess `json:"memberAccess,omitempty"`
}

// ExposeExternallyType describes how Hazelcast members are exposed.
// +kubebuilder:validation:Enum=Smart;Unisocket
type ExposeExternallyType string

const (
	// SmartExposeExternallyType exposes each Hazelcast member with a separate external address.
	SmartExposeExternallyType ExposeExternallyType = "Smart"

	// UnisocketExposeExternallyType exposes all Hazelcast members with one external address.
	UnisocketExposeExternallyType ExposeExternallyType = "Unisocket"
)

// MemberAccess describes how each Hazelcast member is accessed from the external client.
// +kubebuilder:validation:Enum=NodeExternalIP;NodeName;LoadBalancer
type MemberAccess string

const (
	// NodeExternalIPMemberAccess lets the client access Hazelcast member with the NodePort service and the node external IP/hostname
	NodeExternalIPMemberAccess MemberAccess = "NodeExternalIP"

	// NodeNameMemberAccess lets the client access Hazelcast member with the NodePort service and the node name
	NodeNameMemberAccess MemberAccess = "NodeName"

	// LoadBalancerMemberAccess lets the client access Hazelcast member with the LoadBalancer service
	LoadBalancerMemberAccess MemberAccess = "LoadBalancer"
)

// HazelcastStatus defines the observed state of Hazelcast
type HazelcastStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Hazelcast is the Schema for the hazelcasts API
type Hazelcast struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HazelcastSpec   `json:"spec,omitempty"`
	Status HazelcastStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HazelcastList contains a list of Hazelcast
type HazelcastList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Hazelcast `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Hazelcast{}, &HazelcastList{})
}
