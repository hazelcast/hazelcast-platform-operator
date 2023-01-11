package v1alpha1

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Phase represents the current state of the cluster
// +kubebuilder:validation:Enum=Running;Failed;Pending;Terminating
type Phase string

const (
	// Running phase is the state when all the members of the cluster are successfully started
	Running Phase = "Running"
	// Failed phase is the state of error during the cluster startup
	Failed Phase = "Failed"
	// Pending phase is the state of starting the cluster when not all the members are started yet
	Pending Phase = "Pending"
	// Terminating phase is the state where deletion of cluster scoped resources and Hazelcast dependent resources happen
	Terminating Phase = "Terminating"
)

// LoggingLevel controlls log verbosity for Hazelcast.
// +kubebuilder:validation:Enum=OFF;FATAL;ERROR;WARN;INFO;DEBUG;TRACE;ALL
type LoggingLevel string

const (
	LoggingLevelOff   LoggingLevel = "OFF"
	LoggingLevelFatal LoggingLevel = "FATAL"
	LoggingLevelError LoggingLevel = "ERROR"
	LoggingLevelWarn  LoggingLevel = "WARN"
	LoggingLevelInfo  LoggingLevel = "INFO"
	LoggingLevelDebug LoggingLevel = "DEBUG"
	LoggingLevelTrace LoggingLevel = "TRACE"
	LoggingLevelAll   LoggingLevel = "ALL"
)

// HazelcastSpec defines the desired state of Hazelcast
type HazelcastSpec struct {
	// Number of Hazelcast members in the cluster.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=3
	// +optional
	ClusterSize *int32 `json:"clusterSize,omitempty"`

	// Repository to pull the Hazelcast Platform image from.
	// +kubebuilder:default:="docker.io/hazelcast/hazelcast"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version of Hazelcast Platform.
	// +kubebuilder:default:="5.2.1"
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
	// +kubebuilder:default:={}
	// +optional
	ExposeExternally *ExposeExternallyConfiguration `json:"exposeExternally,omitempty"`

	// Name of the Hazelcast cluster.
	// +kubebuilder:default:="dev"
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Scheduling details
	// +kubebuilder:default:={}
	// +optional
	Scheduling SchedulingConfiguration `json:"scheduling,omitempty"`

	// Compute Resources required by the Hazelcast container.
	// +kubebuilder:default:={}
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Persistence configuration
	//+kubebuilder:default:={}
	//+optional
	Persistence *HazelcastPersistenceConfiguration `json:"persistence,omitempty"`

	// B&R Agent configurations
	// +kubebuilder:default:={repository: "docker.io/hazelcast/platform-operator-agent", version: "0.1.13"}
	// +optional
	Agent AgentConfiguration `json:"agent,omitempty"`

	// Jet Engine configuration
	// +kubebuilder:default:={enabled: true, resourceUploadEnabled: false}
	// +optional
	JetEngineConfiguration JetEngineConfiguration `json:"jet,omitempty"`

	// User Codes to Download into CLASSPATH
	//+kubebuilder:default:={}
	// +optional
	UserCodeDeployment UserCodeDeploymentConfig `json:"userCodeDeployment,omitempty"`

	// +optional
	ExecutorServices []ExecutorServiceConfiguration `json:"executorServices,omitempty"`

	// +optional
	DurableExecutorServices []DurableExecutorServiceConfiguration `json:"durableExecutorServices,omitempty"`

	// +optional
	ScheduledExecutorServices []ScheduledExecutorServiceConfiguration `json:"scheduledExecutorServices,omitempty"`

	// +optional
	Properties map[string]string `json:"properties,omitempty"`

	// +kubebuilder:default:="INFO"
	// +optional
	LoggingLevel LoggingLevel `json:"loggingLevel,omitempty"`

	// Configuration to create clusters resilient to node and zone failures
	// +optional
	// +kubebuilder:default:={}
	HighAvailabilityMode HighAvailabilityMode `json:"highAvailabilityMode,omitempty"`
}

// +kubebuilder:validation:Enum=NODE;ZONE
type HighAvailabilityMode string

const (
	HighAvailabilityNodeMode HighAvailabilityMode = "NODE"

	HighAvailabilityZoneMode HighAvailabilityMode = "ZONE"
)

type JetEngineConfiguration struct {
	// When false, disables Jet Engine.
	// +kubebuilder:default:=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// When true, enables resource uploading for Jet jobs.
	// +kubebuilder:default:=false
	// +optional
	ResourceUploadEnabled bool `json:"resourceUploadEnabled"`
}

// Returns true if Jet section is configured.
func (j *JetEngineConfiguration) IsConfigured() bool {
	return j != nil
}

type ExecutorServiceConfiguration struct {
	// The name of the executor service
	// +kubebuilder:default:="default"
	// +optional
	Name string `json:"name,omitempty"`

	// The number of executor threads per member.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=8
	// +optional
	PoolSize int32 `json:"poolSize,omitempty"`

	// Task queue capacity of the executor.
	// +kubebuilder:default:=0
	// +optional
	QueueCapacity int32 `json:"queueCapacity"`
}

type DurableExecutorServiceConfiguration struct {
	// The name of the executor service
	// +kubebuilder:default:="default"
	// +optional
	Name string `json:"name,omitempty"`

	// The number of executor threads per member.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=16
	// +optional
	PoolSize int32 `json:"poolSize,omitempty"`

	// Durability of the executor.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=1
	// +optional
	Durability int32 `json:"durability,omitempty"`

	// Capacity of the executor task per partition.
	// +kubebuilder:default:=100
	// +optional
	Capacity int32 `json:"capacity,omitempty"`
}

type ScheduledExecutorServiceConfiguration struct {
	// The name of the executor service
	// +kubebuilder:default:="default"
	// +optional
	Name string `json:"name,omitempty"`

	// The number of executor threads per member.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=16
	// +optional
	PoolSize int32 `json:"poolSize,omitempty"`

	// Durability of the executor.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:=1
	// +optional
	Durability int32 `json:"durability,omitempty"`

	// Capacity of the executor task per partition.
	// +kubebuilder:default:=100
	// +optional
	Capacity int32 `json:"capacity,omitempty"`

	// The active policy for the capacity setting.
	// +kubebuilder:default:=PER_NODE
	// +optional
	CapacityPolicy string `json:"capacityPolicy,omitempty"`
}

// CapacityPolicyType represents the active policy types for the capacity setting
// +kubebuilder:validation:Enum=PER_NODE;PER_PARTITION
type CapacityPolicyType string

const (
	// CapacityPolicyPerNode is the policy for limiting the maximum number of tasks in each Hazelcast instance
	CapacityPolicyPerNode CapacityPolicyType = "PER_NODE"

	// CapacityPolicyPerPartition is the policy for limiting the maximum number of tasks within each partition.
	CapacityPolicyPerPartition CapacityPolicyType = "PER_PARTITION"
)

type BucketConfiguration struct {
	// Name of the secret with credentials for cloud providers.
	// +kubebuilder:validation:MinLength:=1
	// +required
	Secret string `json:"secret"`

	// Full path to blob storage bucket.
	// +kubebuilder:validation:MinLength:=6
	// +required
	BucketURI string `json:"bucketURI"`
}

// UserCodeDeploymentConfig contains the configuration for User Code download operation
type UserCodeDeploymentConfig struct {
	// When true, allows user code deployment from clients.
	// +optional
	ClientEnabled *bool `json:"clientEnabled,omitempty"`

	// Jar files in the bucket will be put under CLASSPATH.
	// +optional
	BucketConfiguration *BucketConfiguration `json:"bucketConfig,omitempty"`

	// A string for triggering a rolling restart for re-downloading the user code.
	// +optional
	TriggerSequence string `json:"triggerSequence,omitempty"`

	// Files in the ConfigMaps will be put under CLASSPATH.
	// +optional
	ConfigMaps []string `json:"configMaps,omitempty"`
}

// Returns true if userCodeDeployment.bucketConfiguration is specified.
func (c *UserCodeDeploymentConfig) IsBucketEnabled() bool {
	return c != nil && c.BucketConfiguration != nil
}

// Returns true if userCodeDeployment.configMaps configuration is specified.
func (c *UserCodeDeploymentConfig) IsConfigMapEnabled() bool {
	return c != nil && (len(c.ConfigMaps) != 0)
}

type AgentConfiguration struct {
	// Repository to pull Hazelcast Platform Operator Agent(https://github.com/hazelcast/platform-operator-agent)
	// +kubebuilder:default:="docker.io/hazelcast/platform-operator-agent"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version of Hazelcast Platform Operator Agent.
	// +kubebuilder:default:="0.1.13"
	// +optional
	Version string `json:"version,omitempty"`
}

// BackupType represents the storage options for the HotBackup
// +kubebuilder:validation:Enum=External;Local
type BackupType string

const (
	// External backups to the provided cloud provider storage
	External BackupType = "External"

	// Local backups to local storage inside the cluster
	Local BackupType = "Local"
)

// HazelcastPersistenceConfiguration contains the configuration for Hazelcast Persistence and K8s storage.
type HazelcastPersistenceConfiguration struct {
	// Persistence base directory.
	// +required
	BaseDir string `json:"baseDir"`

	// Configuration of the cluster recovery strategy.
	// +kubebuilder:default:="FullRecoveryOnly"
	// +optional
	ClusterDataRecoveryPolicy DataRecoveryPolicyType `json:"clusterDataRecoveryPolicy,omitempty"`

	// StartupAction represents the action triggered when the cluster starts to force the cluster startup.
	// +optional
	StartupAction PersistenceStartupAction `json:"startupAction,omitempty"`

	// DataRecoveryTimeout is timeout for each step of data recovery in seconds.
	// Maximum timeout is equal to DataRecoveryTimeout*2 (for each step: validation and data-load).
	// +optional
	DataRecoveryTimeout int32 `json:"dataRecoveryTimeout,omitempty"`

	// Configuration of PersistenceVolumeClaim.
	// +kubebuilder:default:={}
	// +optional
	Pvc PersistencePvcConfiguration `json:"pvc,omitempty"`

	// Host Path directory.
	// +optional
	HostPath string `json:"hostPath,omitempty"`

	// Restore configuration
	// +kubebuilder:default:={}
	// +optional
	Restore RestoreConfiguration `json:"restore,omitempty"`
}

// Returns true if ClusterDataRecoveryPolicy is not FullRecoveryOnly
func (p *HazelcastPersistenceConfiguration) AutoRemoveStaleData() bool {
	return p.ClusterDataRecoveryPolicy != FullRecovery
}

// Returns true if Persistence configuration is specified.
func (p *HazelcastPersistenceConfiguration) IsEnabled() bool {
	return p != nil && p.BaseDir != ""
}

// Returns true if hostPath is enabled.
func (p *HazelcastPersistenceConfiguration) UseHostPath() bool {
	return p.HostPath != ""
}

// IsRestoreEnabled returns true if Restore configuration is specified
func (p *HazelcastPersistenceConfiguration) IsRestoreEnabled() bool {
	return p.IsEnabled() && !(p.Restore == (RestoreConfiguration{}))
}

// RestoreFromHotBackupResourceName returns true if Restore is done from a HotBackup resource
func (p *HazelcastPersistenceConfiguration) RestoreFromHotBackupResourceName() bool {
	return p.IsRestoreEnabled() && p.Restore.HotBackupResourceName != ""
}

// RestoreConfiguration contains the configuration for Restore operation
// +kubebuilder:validation:MaxProperties=1
type RestoreConfiguration struct {
	// Bucket Configuration from which the backup will be downloaded.
	// +optional
	BucketConfiguration *BucketConfiguration `json:"bucketConfig,omitempty"`

	// Name of the HotBackup resource from which backup will be fetched.
	// +optional
	HotBackupResourceName string `json:"hotBackupResourceName,omitempty"`
}

func (rc RestoreConfiguration) Hash() string {
	str, _ := json.Marshal(rc)
	return strconv.Itoa(int(FNV32a(string(str))))
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

func (pvc PersistencePvcConfiguration) IsEmpty() bool {
	return pvc.AccessModes == nil && pvc.RequestStorage == nil && pvc.StorageClassName == nil
}

// DataRecoveryPolicyType represents the options for data recovery policy when the whole cluster restarts.
// +kubebuilder:validation:Enum=FullRecoveryOnly;PartialRecoveryMostRecent;PartialRecoveryMostComplete
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
)

// PersistenceStartupAction represents the action triggered on the cluster startup to force the cluster startup.
// +kubebuilder:validation:Enum=ForceStart;PartialStart
type PersistenceStartupAction string

const (
	// ForceStart will trigger the force start action on the startup
	ForceStart PersistenceStartupAction = "ForceStart"

	// PartialStart will trigger the partial start action on the startup.
	// Can be used only with the MostComplete or MostRecent DataRecoveryPolicyType type.
	PartialStart PersistenceStartupAction = "PartialStart"
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
	// +kubebuilder:default:="Smart"
	// +optional
	Type ExposeExternallyType `json:"type,omitempty"`

	// Type of the service used to discover Hazelcast cluster.
	// +kubebuilder:default:="LoadBalancer"
	// +optional
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

// HazelcastStatus defines the observed state of Hazelcast
type HazelcastStatus struct {
	// Phase of the Hazelcast cluster
	// +optional
	Phase Phase `json:"phase,omitempty"`

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
	// +optional
	Members []HazelcastMemberStatus `json:"members,omitempty"`

	// Status of restore process of the Hazelcast cluster
	// +kubebuilder:default:={}
	// +optional
	Restore RestoreStatus `json:"restore,omitempty"`
}

// +kubebuilder:validation:Enum=Unknown;Failed;InProgress;Succeeded
type RestoreState string

const (
	RestoreUnknown    RestoreState = "Unknown"
	RestoreFailed     RestoreState = "Failed"
	RestoreInProgress RestoreState = "InProgress"
	RestoreSucceeded  RestoreState = "Succeeded"
)

type RestoreStatus struct {
	// State shows the current phase of the restore process of the cluster.
	// +optional
	State RestoreState `json:"state,omitempty"`

	// RemainingValidationTime show the time in seconds remained for the restore validation step.
	// +optional
	RemainingValidationTime int64 `json:"remainingValidationTime,omitempty"`

	// RemainingDataLoadTime show the time in seconds remained for the restore data load step.
	// +optional
	RemainingDataLoadTime int64 `json:"remainingDataLoadTime,omitempty"`
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

	// State represents the observed state of the member.
	// +optional
	State NodeState `json:"state,omitempty"`

	// Master flag is set to true if the member is master.
	// +optional
	Master bool `json:"master,omitempty"`

	// Lite is the flag that is true when the member is lite-member.
	// +optional
	Lite bool `json:"lite,omitempty"`

	// OwnedPartitions represents the partitions count on the member.
	// +optional
	OwnedPartitions int32 `json:"ownedPartitions,omitempty"`

	// Ready is the flag that is set to true when the member is successfully started,
	// connected to cluster and ready to accept connections.
	// +optional
	Ready bool `json:"connected"`

	// Message contains the optional message with the details of the cluster state.
	// +optional
	Message string `json:"message,omitempty"`

	// Reason contains the optional reason of member crash or restart.
	// +optional
	Reason string `json:"reason,omitempty"`

	// RestartCount is the number of times the member has been restarted.
	// +optional
	RestartCount int32 `json:"restartCount"`
}

// +kubebuilder:validation:Enum=PASSIVE;ACTIVE;SHUT_DOWN;STARTING
type NodeState string

const (
	NodeStatePassive  NodeState = "PASSIVE"
	NodeStateActive   NodeState = "ACTIVE"
	NodeStateShutDown NodeState = "SHUT_DOWN"
	NodeStateStarting NodeState = "STARTING"
)

// HazelcastClusterStatus defines the status of the Hazelcast cluster
type HazelcastClusterStatus struct {
	// ReadyMembers represents the number of members that are connected to cluster from the desired number of members
	// in the format <ready>/<desired>
	// +optional
	ReadyMembers string `json:"readyMembers,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Hazelcast is the Schema for the hazelcasts API
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the Hazelcast deployment"
// +kubebuilder:printcolumn:name="Members",type="string",JSONPath=".status.hazelcastClusterStatus.readyMembers",description="Current numbers of ready Hazelcast members"
// +kubebuilder:printcolumn:name="External-Addresses",type="string",JSONPath=".status.externalAddresses",description="External addresses of the Hazelcast cluster"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Hazelcast Config"
// +kubebuilder:resource:shortName=hz
type Hazelcast struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Initial values will be filled with its fields' default values.
	// +kubebuilder:default:={"repository" : "docker.io/hazelcast/hazelcast"}
	// +optional
	Spec HazelcastSpec `json:"spec,omitempty"`

	// +optional
	Status HazelcastStatus `json:"status,omitempty"`
}

func (h *Hazelcast) DockerImage() string {
	return fmt.Sprintf("%s:%s", h.Spec.Repository, h.Spec.Version)
}

func (h *Hazelcast) ClusterScopedName() string {
	return fmt.Sprintf("%s-%d", h.Name, FNV32a(h.Namespace))
}

func (h *Hazelcast) ExternalAddressEnabled() bool {
	return h.Spec.ExposeExternally.IsEnabled() &&
		h.Spec.ExposeExternally.DiscoveryServiceType == corev1.ServiceTypeLoadBalancer
}

func (h *Hazelcast) AgentDockerImage() string {
	return fmt.Sprintf("%s:%s", h.Spec.Agent.Repository, h.Spec.Agent.Version)
}

//+kubebuilder:object:root=true

// HazelcastList contains a list of Hazelcast
type HazelcastList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Hazelcast `json:"items"`
}

func FNV32a(txt string) uint32 {
	alg := fnv.New32a()
	alg.Write([]byte(txt))
	return alg.Sum32()
}

func init() {
	SchemeBuilder.Register(&Hazelcast{}, &HazelcastList{})
}
