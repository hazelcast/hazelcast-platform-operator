package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TurbineSpec defines the desired state of Turbine
type TurbineSpec struct {
	// Sidecar configuration
	// +kubebuilder:default:={"name" : "turbine-sidecar"}
	Sidecar *SidecarConfiguration `json:"sidecar,omitempty"`

	// Hazelcast cluster to connect
	Hazelcast *HazelcastReference `json:"hazelcast,omitempty"`

	// Pod configuration
	Pods *PodConfiguration `json:"pods,omitempty"`
}

type SidecarConfiguration struct {
	// Name of the Turbine sidecar container
	// +kubebuilder:default:="turbine-sidecar"
	Name string `json:"name,omitempty"`

	// Repository of the image used for Turbine sidecar container
	// +kubebuilder:default:="hazelcast/turbine-sidecar"
	Repository string `json:"repository,omitempty"`

	// Version of the image used for Turbine sidecar container
	// +kubebuilder:default:="latest"
	Version string `json:"version,omitempty"`
}

// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:MaxProperties=1
type HazelcastReference struct {
	// Address of the Hazelcast cluster
	ClusterAddress *string `json:"clusterAddress,omitempty"`

	// Reference to Hazelcast CR
	Cluster *HazelcastRef `json:"cluster,omitempty"`
}

type HazelcastRef struct {
	// Name of Hazelcast CR
	Name string `json:"name"`

	// Namespace of Hazelcast CR
	// +kubebuilder:default:="default"
	Namespace string `json:"namespace,omitempty"`
}

type PodConfiguration struct {
	// The name of the container port that Turbine will register
	AppPortName *string `json:"portName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster

// Turbine is the Schema for the turbines API
type Turbine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TurbineSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// TurbineList contains a list of Turbine
type TurbineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Turbine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Turbine{}, &TurbineList{})
}
