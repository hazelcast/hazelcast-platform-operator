package v1alpha1

import (
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Phase represents the current state of the cluster
// +kubebuilder:validation:Enum=Running;Failed;Pending
type Phase string

const (
	// Running phase is the state when all the members of the cluster are successfully started
	Running Phase = "Running"
	// Failed phase is the state of error during the cluster startup
	Failed Phase = "Failed"
	// Pending phase is the state of starting the cluster when not all the members are started yet
	Pending Phase = "Pending"
)

// HazelcastSpec defines the desired state of Hazelcast
type HazelcastSpec struct {
	// Number of Hazelcast members in the cluster.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=3
	// +optional
	// TODO should we let minimum to be 0?
	ClusterSize int32 `json:"clusterSize,omitempty"`

	// Repository to pull the Hazelcast Platform image from.
	// +kubebuilder:default:="docker.io/hazelcast/hazelcast"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version of Hazelcast Platform.
	// +kubebuilder:default:="5.1"
	// +optional
	Version string `json:"version,omitempty"`

	// Pull policy for the Hazelcast Platform image
	// +kubebuilder:default:="IfNotPresent"
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Image pull secrets for the Hazelcast Platform image
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +optional
	LicenseKeySecret string `json:"licenseKeySecret,omitempty"`

	// Configuration to expose Hazelcast cluster to external clients.
	// +optional
	// +kubebuilder:default:={}
	ExposeExternally *ExposeExternallyConfiguration `json:"exposeExternally,omitempty"`

	// Name of the Hazelcast cluster.
	// +kubebuilder:default:="dev"
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Scheduling details
	// +optional
	// +kubebuilder:default:={}
	Scheduling *SchedulingConfiguration `json:"scheduling,omitempty"`

	// Persistence configuration
	//+optional
	//+kubebuilder:default:={}
	Persistence *HazelcastPersistenceConfiguration `json:"persistence,omitempty"`
}

// HazelcastPersistenceConfiguration contains the configuration for Hazelcast Persistence and K8s storage.
type HazelcastPersistenceConfiguration struct {

	// Persistence base directory.
	BaseDir string `json:"baseDir"`

	// Configuration of the cluster recovery strategy.
	// +kubebuilder:default:="FullRecoveryOnly"
	// +optional
	ClusterDataRecoveryPolicy DataRecoveryPolicyType `json:"clusterDataRecoveryPolicy,omitempty"`

	// Configuration of PersistenceVolumeClaim.
	// +optional
	Pvc PersistencePvcConfiguration `json:"pvc"`

	// Host Path directory.
	// +optional
	HostPath string `json:"hostPath,omitempty"`
}

type PersistencePvcConfiguration struct {
	// AccessModes contains the actual access modes of the volume backing the PVC has.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// A description of the PVC request capacity.
	// +optional
	RequestStorage *resource.Quantity `json:"requestStorage,omitempty"`

	// Name of StorageClass which this persistent volume belongs to.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// DataRecoveryPolicyType represents the options for data recovery policy when the whole cluster restarts.
// +kubebuilder:validation:Enum=FullRecoveryOnly;PartialRecoveryMostRecent;PartialRecoveryMostComplete;PartialRecoveryForceStart
type DataRecoveryPolicyType string

const (
	// FullRecovery does not allow partial start of the cluster
	// and corresponds to "cluster-data-recovery-policy.FULL_RECOVERY_ONLY" configuration option.
	FullRecovery DataRecoveryPolicyType = "FullRecoveryOnly"

	// MostRecent allow partial start with the members that have most up-to-date partition table
	// and corresponds to "cluster-data-recovery-policy.PARTIAL_RECOVERY_MOST_RECENT" configuration option.
	MostRecent DataRecoveryPolicyType = "PartialRecoveryMostRecent"

	// MostComplete allow partial start with the members that have most complete partition table
	// and corresponds to "cluster-data-recovery-policy.PARTIAL_RECOVERY_MOST_COMPLETE" configuration option.
	MostComplete DataRecoveryPolicyType = "PartialRecoveryMostComplete"

	// ForceStart deletes all data in your cluster members' persistence stores when the cluster recovery fails.
	ForceStart DataRecoveryPolicyType = "ForceStart"
)

// SchedulingConfiguration defines the pods scheduling details
type SchedulingConfiguration struct {
	// Affinity
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// TopologySpreadConstraints
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// ExposeExternallyConfiguration defines how to expose Hazelcast cluster to external clients
type ExposeExternallyConfiguration struct {
	// Specifies how members are exposed.
	// Valid values are:
	// - "Smart" (default): each member pod is exposed with a separate external address
	// - "Unisocket": all member pods are exposed with one external address
	// +optional
	// +kubebuilder:default:="Smart"
	Type ExposeExternallyType `json:"type,omitempty"`

	// Type of the service used to discover Hazelcast cluster.
	// +optional
	// +kubebuilder:default:="LoadBalancer"
	DiscoveryServiceType corev1.ServiceType `json:"discoveryServiceType,omitempty"`

	// How each member is accessed from the external client.
	// Only available for "Smart" client and valid values are:
	// - "NodePortExternalIP" (default): each member is accessed by the NodePort service and the node external IP/hostname
	// - "NodePortNodeName": each member is accessed by the NodePort service and the node name
	// - "LoadBalancer": each member is accessed by the LoadBalancer service external address
	// +optional
	MemberAccess MemberAccess `json:"memberAccess,omitempty"`
}

// ExposeExternallyType describes how Hazelcast members are exposed.
// +kubebuilder:validation:Enum=Smart;Unisocket
type ExposeExternallyType string

const (
	// ExposeExternallyTypeSmart exposes each Hazelcast member with a separate external address.
	ExposeExternallyTypeSmart ExposeExternallyType = "Smart"

	// ExposeExternallyTypeUnisocket exposes all Hazelcast members with one external address.
	ExposeExternallyTypeUnisocket ExposeExternallyType = "Unisocket"
)

// MemberAccess describes how each Hazelcast member is accessed from the external client.
// +kubebuilder:validation:Enum=NodePortExternalIP;NodePortNodeName;LoadBalancer
type MemberAccess string

const (
	// MemberAccessNodePortExternalIP lets the client access Hazelcast member with the NodePort service and the node external IP/hostname
	MemberAccessNodePortExternalIP MemberAccess = "NodePortExternalIP"

	// MemberAccessNodePortNodeName lets the client access Hazelcast member with the NodePort service and the node name
	MemberAccessNodePortNodeName MemberAccess = "NodePortNodeName"

	// MemberAccessLoadBalancer lets the client access Hazelcast member with the LoadBalancer service
	MemberAccessLoadBalancer MemberAccess = "LoadBalancer"
)

func (p *HazelcastPersistenceConfiguration) AutoRemoveStaleData() bool {
	return p.ClusterDataRecoveryPolicy != FullRecovery
}

// Returns true if exposeExternally configuration is specified.
func (c *ExposeExternallyConfiguration) IsEnabled() bool {
	return c != nil && !(*c == (ExposeExternallyConfiguration{}))
}

// Returns true if Smart configuration is specified and therefore each Hazelcast member needs to be exposed with a separate address.
func (c *ExposeExternallyConfiguration) IsSmart() bool {
	return c != nil && c.Type == ExposeExternallyTypeSmart
}

// Returns true if Hazelcast client wants to use Node Name instead of External IP.
func (c *ExposeExternallyConfiguration) UsesNodeName() bool {
	return c != nil && c.MemberAccess == MemberAccessNodePortNodeName
}

// Returns service type that is used for the cluster discovery (LoadBalancer by default).
func (c *ExposeExternallyConfiguration) DiscoveryK8ServiceType() corev1.ServiceType {
	if c == nil {
		return corev1.ServiceTypeLoadBalancer
	}

	switch c.DiscoveryServiceType {
	case corev1.ServiceTypeNodePort:
		return corev1.ServiceTypeNodePort
	default:
		return corev1.ServiceTypeLoadBalancer
	}
}

// Returns the member access type that is used for the communication with each member (NodePortExternalIP by default).
func (c *ExposeExternallyConfiguration) MemberAccessType() MemberAccess {
	if c == nil {
		return MemberAccessNodePortExternalIP
	}

	if c.MemberAccess != "" {
		return c.MemberAccess
	}
	return MemberAccessNodePortExternalIP
}

// Returns service type that is used for the communication with each member (NodePort by default).
func (c *ExposeExternallyConfiguration) MemberAccessServiceType() corev1.ServiceType {
	if c == nil {
		return corev1.ServiceTypeNodePort
	}

	switch c.MemberAccess {
	case MemberAccessLoadBalancer:
		return corev1.ServiceTypeLoadBalancer
	default:
		return corev1.ServiceTypeNodePort
	}
}

// Returns true if Persistence configuration is specified.
func (c *HazelcastPersistenceConfiguration) IsEnabled() bool {
	return c != nil && c.BaseDir != ""
}

// Returns true if hostPath is enabled.
func (c *HazelcastPersistenceConfiguration) UseHostPath() bool {
	return c.HostPath != ""
}

// HazelcastStatus defines the observed state of Hazelcast
type HazelcastStatus struct {
	// Phase of the Hazelcast cluster
	// +optional
	Phase Phase `json:"phase"`

	// Status of the Hazelcast cluster
	// +optional
	Cluster HazelcastClusterStatus `json:"hazelcastClusterStatus,omitempty"`

	// Message about the Hazelcast cluster state
	// +optional
	Message string `json:"message,omitempty"`

	// External addresses of the Hazelcast cluster members
	// +optional
	ExternalAddresses string `json:"externalAddresses,omitempty"`

	// Status of Hazelcast members
	// + optional
	Members []HazelcastMemberStatus `json:"members,omitempty"`
}

// HazelcastMemberStatus defines the observed state of the individual Hazelcast member.
type HazelcastMemberStatus struct {

	// PodName is the name of the Hazelcast member pod.
	// +optional
	PodName string `json:"podName,omitempty"`

	// Uid is the unique member identifier within the cluster.
	// +optional
	Uid string `json:"uid,omitempty"`

	// Ip is the IP address of the member within the cluster.
	// +optional
	Ip string `json:"ip,omitempty"`

	// Version represents the Hazelcast version of the member.
	// +optional
	Version string `json:"version,omitempty"`

	// Ready is the flag that is set to true when the member is successfully started,
	// connected to cluster and ready to accept connections.
	Ready bool `json:"connected"`

	// Message contains the optional message with the details of the cluster state.
	// +optional
	Message string `json:"message,omitempty"`

	// Reason contains the optional reason of member crash or restart.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// HazelcastClusterStatus defines the status of the Hazelcast cluster
type HazelcastClusterStatus struct {
	// ReadyMembers represents the number of members that are connected to cluster from the desired number of members
	// in the format <ready>/<desired>
	ReadyMembers string `json:"readyMembers"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Hazelcast is the Schema for the hazelcasts API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the Hazelcast deployment"
// +kubebuilder:printcolumn:name="Members",type="string",JSONPath=".status.hazelcastClusterStatus.readyMembers",description="Current numbers of ready Hazelcast members"
// +kubebuilder:printcolumn:name="External-Addresses",type="string",JSONPath=".status.externalAddresses",description="External addresses of the Hazelcast cluster"
type Hazelcast struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	// +kubebuilder:default:={"repository" : "docker.io/hazelcast/hazelcast"}
	Spec HazelcastSpec `json:"spec,omitempty"`
	// +optional
	Status HazelcastStatus `json:"status,omitempty"`
}

func (h *Hazelcast) DockerImage() string {
	return fmt.Sprintf("%s:%s", h.Spec.Repository, h.Spec.Version)
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

func FNV32a(txt string) uint32 {
	alg := fnv.New32a()
	alg.Write([]byte(txt))
	return alg.Sum32()
}

func (h *Hazelcast) ClusterScopedName() string {
	return fmt.Sprintf("%s-%d", h.Name, FNV32a(h.Namespace))
}

func (h *Hazelcast) ExternalAddressEnabled() bool {
	return h.Spec.ExposeExternally.IsEnabled() &&
		h.Spec.ExposeExternally.DiscoveryServiceType == corev1.ServiceTypeLoadBalancer
}
