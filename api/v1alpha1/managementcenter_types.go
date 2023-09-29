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
	Repository string `json:"repository,omitempty"`

	// Version of Management Center.
	// +kubebuilder:default:="5.3.2"
	// +optional
	Version string `json:"version,omitempty"`

	// Pull policy for the Management Center image
	// +kubebuilder:default:="IfNotPresent"
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Image pull secrets for the Management Center image
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// licenseKeySecret is a deprecated alias for licenseKeySecretName.
	// +optional
	DeprecatedLicenseKeySecret string `json:"licenseKeySecret,omitempty"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +optional
	LicenseKeySecretName string `json:"licenseKeySecretName,omitempty"`

	// Connection configuration for the Hazelcast clusters that Management Center will monitor.
	// +optional
	HazelcastClusters []HazelcastClusterConfig `json:"hazelcastClusters,omitempty"`

	// Configuration to expose Management Center to outside.
	// +kubebuilder:default:={type: "LoadBalancer"}
	// +optional
	ExternalConnectivity *ExternalConnectivityConfiguration `json:"externalConnectivity,omitempty"`

	// Configuration for Management Center persistence.
	// +kubebuilder:default:={enabled: true, size: "10Gi"}
	// +optional
	Persistence *MCPersistenceConfiguration `json:"persistence,omitempty"`

	// Scheduling details
	// +kubebuilder:default:={}
	// +optional
	Scheduling *SchedulingConfiguration `json:"scheduling,omitempty"`

	// Compute Resources required by the MC container.
	// +kubebuilder:default:={}
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ManagementCenter JVM configuration
	// +optional
	JVM *MCJVMConfiguration `json:"jvm,omitempty"`

	// SecurityProviders to authenticate users in Management Center
	// +optional
	SecurityProviders *SecurityProviders `json:"securityProvider,omitempty"`
}

func (s *ManagementCenterSpec) GetLicenseKeySecretName() string {
	if s.LicenseKeySecretName == "" {
		return s.DeprecatedLicenseKeySecret
	}
	return s.LicenseKeySecretName
}

type SecurityProviders struct {
	// LDAP security provider
	// +optional
	LDAP *LDAPProvider `json:"ldap,omitempty"`
}

type LDAPProvider struct {
	// URL of your LDAP server, including schema (ldap:// or ldaps://) and port.
	// +required
	URL string `json:"url"`

	// CredentialsSecretName is the name of the secret that contains username and password of a user that has admin privileges on the LDAP server.
	// The username must be the DN of the user. It is used to connect to the server when authenticating users.
	// +required
	CredentialsSecretName string `json:"credentialsSecretName"`

	// DN to be used for searching users.
	// +required
	UserDN string `json:"userDn"`

	// DN to be used for searching groups.
	// +required
	GroupDN string `json:"groupDn"`

	// Members of these groups and its nested groups have admin privileges on the Management Center.
	// +kubebuilder:validation:MinItems:=1
	AdminGroups []string `json:"adminGroups"`

	// Members of these groups and its nested groups have read and write privileges on the Management Center.
	// +kubebuilder:validation:MinItems:=1
	UserGroups []string `json:"userGroups"`

	// Members of these groups and its nested groups have only read privilege on the Management Center.
	// +kubebuilder:validation:MinItems:=1
	ReadonlyUserGroups []string `json:"readonlyUserGroups"`

	// Members of these groups and its nested groups have the privilege to see only the metrics on the Management Center.
	// +kubebuilder:validation:MinItems:=1
	MetricsOnlyGroups []string `json:"metricsOnlyGroups"`

	// LDAP search filter expression to search for the users.
	// For example, uid={0} searches for a username that matches with the uid attribute.
	// +required
	UserSearchFilter string `json:"userSearchFilter"`

	// LDAP search filter expression to search for the groups.
	// For example, uniquemember={0} searches for a group that matches with the uniquemember attribute.
	// +required
	GroupSearchFilter string `json:"groupSearchFilter"`

	// NestedGroupSearch enables searching for nested LDAP groups.
	// +kubebuilder:default:=false
	NestedGroupSearch bool `json:"nestedGroupSearch"`
}

type HazelcastClusterConfig struct {
	// Name of the Hazelcast cluster that Management Center will connect to, default is dev.
	// +kubebuilder:default:="dev"
	// +optional
	Name string `json:"name,omitempty"`

	// IP address or DNS name of the Hazelcast cluster.
	// If the cluster is exposed with a service name in a different namespace, use the following syntax "<service-name>.<service-namespace>".
	// +required
	Address string `json:"address"`

	// TLS client configuration.
	// +kubebuilder:default:={}
	// +optional
	TLS *TLS `json:"tls,omitempty"`
}

// ExternalConnectivityConfiguration defines how to expose Management Center pod.
type ExternalConnectivityConfiguration struct {
	// How Management Center is exposed.
	// Valid values are:
	// - "ClusterIP"
	// - "NodePort"
	// - "LoadBalancer" (default)
	// +kubebuilder:default:="LoadBalancer"
	// +optional
	Type ExternalConnectivityType `json:"type,omitempty"`

	// Ingress configuration of Management Center
	// +optional
	Ingress *ExternalConnectivityIngress `json:"ingress,omitempty"`

	// OpenShift Route configuration of Management Center
	// +optional
	Route *ExternalConnectivityRoute `json:"route,omitempty"`
}

// ManagementCenterServiceType returns service type that is used by Management Center (LoadBalancer by default).
func (ec *ExternalConnectivityConfiguration) ManagementCenterServiceType() corev1.ServiceType {
	if ec == nil {
		return corev1.ServiceTypeClusterIP
	}

	switch ec.Type {
	case ExternalConnectivityTypeClusterIP:
		return corev1.ServiceTypeClusterIP
	case ExternalConnectivityTypeNodePort:
		return corev1.ServiceTypeNodePort
	default:
		return corev1.ServiceTypeLoadBalancer
	}
}

// IsEnabled returns true if external connectivity is enabled.
func (ec *ExternalConnectivityConfiguration) IsEnabled() bool {
	return ec != nil && *ec != ExternalConnectivityConfiguration{}
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

// ExternalConnectivityIngress defines ingress configuration of Management Center
type ExternalConnectivityIngress struct {
	// Hostname of Management Center exposed by Ingress.
	// Ingress controller will use this hostname to route inbound traffic.
	// +required
	Hostname string `json:"hostname"`

	// IngressClassName of the ingress object.
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`

	// Annotations added to the ingress object.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ExternalConnectivityRoute defines OpenShift route configuration of Management Center
type ExternalConnectivityRoute struct {
	// Hostname of Management Center exposed by route.
	// Openshift routers will use this hostname to route inbound traffic.
	Hostname string `json:"hostname"`
}

// IsEnabled returns true if external connectivity ingress is enabled.
func (eci *ExternalConnectivityIngress) IsEnabled() bool {
	return eci != nil
}

// IsEnabled returns true if external connectivity ingress is enabled.
func (ecr *ExternalConnectivityRoute) IsEnabled() bool {
	return ecr != nil
}

type MCPersistenceConfiguration struct {
	// When true, MC will use a PersistentVolumeClaim to store data.
	// +kubebuilder:default:=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Name of the PersistentVolumeClaim MC will use for persistence. If not empty,
	// MC will use the existing claim instead of creating a new one.
	// +optional
	ExistingVolumeClaimName string `json:"existingVolumeClaimName,omitempty"`

	// StorageClass from which PersistentVolumeClaim will be created.
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`

	// Size of the created PersistentVolumeClaim.
	// +kubebuilder:default:="10Gi"
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`
}

// IsEnabled returns true if persistence configuration is specified.
func (pc *MCPersistenceConfiguration) IsEnabled() bool {
	return pc != nil && pc.Enabled != nil && *pc.Enabled
}

// MCJVMConfiguration is a ManagementCenter JVM configuration
type MCJVMConfiguration struct {
	// Args is for arbitrary JVM arguments
	// +optional
	Args []string `json:"args,omitempty"`
}

func (c *MCJVMConfiguration) IsConfigured() bool {
	return c != nil && c.Args != nil && len(c.Args) > 0
}

// McPhase represents the current state of the cluster
// +kubebuilder:validation:Enum=Running;Failed;Pending;Terminating
type McPhase string

const (
	// Running phase is the state when the ManagementCenter is successfully started
	McRunning McPhase = "Running"
	// Failed phase is the state of error during the ManagementCenter startup
	McFailed McPhase = "Failed"
	// Pending phase is the state of starting the cluster when the ManagementCenter is not started yet
	McPending McPhase = "Pending"
	// Terminating phase is the state where deletion of ManagementCenter and dependent resources happen
	McTerminating McPhase = "Terminating"
)

// ManagementCenterStatus defines the observed state of ManagementCenter.
type ManagementCenterStatus struct {
	// Phase of the Management Center
	// +optional
	Phase McPhase `json:"phase,omitempty"`

	// Message about the Management Center state
	// +optional
	Message string `json:"message,omitempty"`

	// External addresses of the Management Center instance
	// +optional
	ExternalAddresses string `json:"externalAddresses,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ManagementCenter is the Schema for the managementcenters API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the Management Center deployment"
// +kubebuilder:printcolumn:name="External-Addresses",type="string",JSONPath=".status.externalAddresses",description="External addresses of the Management Center deployment"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current ManagementCenter Config"
// +kubebuilder:resource:shortName=mc
type ManagementCenter struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Initial values will be filled with its fields' default values.
	// +kubebuilder:default:={"repository" : "docker.io/hazelcast/management-center"}
	// +optional
	Spec ManagementCenterSpec `json:"spec,omitempty"`

	// +optional
	Status ManagementCenterStatus `json:"status,omitempty"`
}

func (mc *ManagementCenter) DockerImage() string {
	return fmt.Sprintf("%s:%s", mc.Spec.Repository, mc.Spec.Version)
}

func (mc *ManagementCenter) ExternalAddressEnabled() bool {
	return mc.Spec.ExternalConnectivity.IsEnabled() &&
		mc.Spec.ExternalConnectivity.Type == ExternalConnectivityTypeLoadBalancer
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
