package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HazelcastSpec defines the desired state of Hazelcast
type ManagementCenterSpec struct {
	// Repository to pull the Hazelcast Platform image from.
	// +kubebuilder:default:="docker.io/hazelcast/management-center"
	// +optional
	Repository string `json:"repository"`

	// Version of Hazelcast Platform.
	// +kubebuilder:default:="5.0-BETA-2"
	// +optional
	Version string `json:"version"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +kubebuilder:default:="hazelcast-license-key"
	// +optional
	LicenseKeySecret string `json:"licenseKeySecret"`

	// Connection configuration for Hazelcast Clusters Management Center will monitor.
	// +optional
	HazelcastClusters []HazelcastClusterConfig `json:"hazelcastClusters"`

	// Configuration to expose Management Center to outside.
	// +optional
	ExternalConnectivity ExternalConnectivityConfiguration `json:"externalConnectivity"`

	// Configuration for Management Center persistence
	// +optional
	Persistence PersistenceConfiguration `json:"persistence"`
}

type HazelcastClusterConfig struct {
	// Name of the Hazelcast cluster Management Center will connect to
	// +optional
	Name string `json:"name"`
	// Address of one of the members of Hazelcast cluster
	// Can be IP address or DNS name. For Hazelcast cluster exposed with a service name in
	// different namespace address can be given as "<service-name>.<service-namespace>."
	Address string `json:"address"`
}

// ExternalConnectivityConfiguration defines how to expose Management Center pod
type ExternalConnectivityConfiguration struct {
	// Specifies how Management Center is exposed
	// Valid values are:
	// - "ClusterIP"
	// - "NodePort"
	// - "Loadbalancer" (default)
	// +optional
	// +kubebuilder:default:="LoadBalancer"
	Type ExternalConnectivityType `json:"type,omitempty"`
}

// ExternalConnectivityType describes how Management Center is exposed.
// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer
type ExternalConnectivityType string

const (
	// ExternalConnectivityTypeClusterIP exposes Management Center with ClusterIP service.
	ExternalConnectivityTypeClusterIP ExternalConnectivityType = "ClusterIP"

	// ExternalConnectivityTypeNodePort exposes Management Center with NodePort service.
	ExternalConnectivityTypeNodePort ExternalConnectivityType = "NodePort"

	// ExternalConnectivityTypeLoadBalancer exposes Management Center with LoadBalancer service.
	ExternalConnectivityTypeLoadBalancer ExternalConnectivityType = "LoadBalancer"
)

type PersistenceConfiguration struct {
	// +optional
	// +kubebuilder:default:=true
	Enabled bool `json:"enabled"`

	// +optional
	StorageClass *string `json:"storageClass"`

	// +optional
	// +kubebuilder:default:="10Gi"
	Size resource.Quantity `json:"size"`
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

// Returns service type that is used by Management Center (LoadBalancer by default).
func (c *ExternalConnectivityConfiguration) ManagementCenterServiceType() corev1.ServiceType {
	switch c.Type {
	case ExternalConnectivityTypeClusterIP:
		return corev1.ServiceTypeClusterIP
	case ExternalConnectivityTypeNodePort:
		return corev1.ServiceTypeNodePort
	default:
		return corev1.ServiceTypeLoadBalancer
	}
}

// Returns true if persistence configuration is specified.
func (c *PersistenceConfiguration) IsEnabled() bool {
	return c.Enabled
}
