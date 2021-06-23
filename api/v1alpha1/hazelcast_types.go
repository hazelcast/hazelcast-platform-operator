package v1alpha1

import (
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
	// Name of the secret with Hazelcast Enterprise License Key
	// +kubebuilder:default:="hazelcast-license-key"
	// +optional
	LicenseKeySecret string `json:"licenseKeySecret"`
}

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
