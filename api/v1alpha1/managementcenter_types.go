package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagementCenterSpec defines the desired state of ManagementCenter.
type ManagementCenterSpec struct {
	// Repository to pull the Management Center image from.
	// +kubebuilder:default:="docker.io/hazelcast/management-center"
	// +optional
	Repository string `json:"repository"`

	// Version of Management Center.
	// +kubebuilder:default:="5.0.4"
	// +optional
	Version string `json:"version"`

	// Pull policy for the Management Center image
	// +kubebuilder:default:="IfNotPresent"
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// Image pull secrets for the Management Center image
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +optional
	LicenseKeySecret string `json:"licenseKeySecret"`

	// Connection configuration for the Hazelcast clusters that Management Center will monitor.
	// +optional
	HazelcastClusters []HazelcastClusterConfig `json:"hazelcastClusters,omitempty"`

	// Configuration to expose Management Center to outside.
	// +optional
	ExternalConnectivity ExternalConnectivityConfiguration `json:"externalConnectivity"`

	// Configuration for Management Center persistence.
	// +optional
	// +kubebuilder:default:={enabled: true, size: "10Gi"}
	Persistence PersistenceConfiguration `json:"persistence,omitempty"`
}

type HazelcastClusterConfig struct {
	// Name of the Hazelcast cluster that Management Center will connect to, default is dev.
	// +optional
	// +kubebuilder:default:="dev"
	Name string `json:"name"`
	// IP address or DNS name of the Hazelcast cluster.
	// If the cluster is exposed with a service name in a different namespace, use the following syntax "<service-name>.<service-namespace>".
	Address string `json:"address"`
}

// ExternalConnectivityConfiguration defines how to expose Management Center pod.
type ExternalConnectivityConfiguration struct {
	// How Management Center is exposed.
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
	// When true, MC will use a PersistentVolumeClaim to store data.
	// +optional
	// +kubebuilder:default:=true
	Enabled bool `json:"enabled"`

	// Name of the PersistentVolumeClaim MC will use for persistence. If not empty,
	// MC will use the existing claim instead of creating a new one.
	// +optional
	ExistingVolumeClaimName string `json:"existingVolumeClaimName,omitempty"`

	// StorageClass from which PersistentVolumeClaim will be created.
	// +optional
	// +nullable
	StorageClass *string `json:"storageClass"`

	// Size of the created PersistentVolumeClaim.
	// +optional
	// +kubebuilder:default:="10Gi"
	Size resource.Quantity `json:"size"`
}

// ManagementCenterStatus defines the observed state of ManagementCenter.
type ManagementCenterStatus struct {
	Phase   Phase  `json:"phase"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ManagementCenter is the Schema for the managementcenters API
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the Management Center deployment"
type ManagementCenter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagementCenterSpec   `json:"spec,omitempty"`
	Status ManagementCenterStatus `json:"status,omitempty"`
}

func (mc *ManagementCenter) DockerImage() string {
	return fmt.Sprintf("%s:%s", mc.Spec.Repository, mc.Spec.Version)
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
