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

// LoggingLevel controls log verbosity for Hazelcast.
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
	// +kubebuilder:default:="docker.io/hazelcast/hazelcast-enterprise"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version of Hazelcast Platform.
	// +kubebuilder:default:="5.5.0-SNAPSHOT"
	// +optional
	Version string `json:"version,omitempty"`

	// Pull policy for the Hazelcast Platform image
	// +kubebuilder:default:="IfNotPresent"
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Image pull secrets for the Hazelcast Platform image
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// licenseKeySecret is a deprecated alias for licenseKeySecretName.
	// +optional
	DeprecatedLicenseKeySecret string `json:"licenseKeySecret,omitempty"`

	// Name of the secret with Hazelcast Enterprise License Key.
	// +kubebuilder:validation:MinLength:=1
	// +required
	LicenseKeySecretName string `json:"licenseKeySecretName"`

	// Configuration to expose Hazelcast cluster to external clients.
	// +optional
	ExposeExternally *ExposeExternallyConfiguration `json:"exposeExternally,omitempty"`

	// Name of the Hazelcast cluster.
	// +kubebuilder:default:="dev"
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Scheduling details
	// +optional
	Scheduling *SchedulingConfiguration `json:"scheduling,omitempty"`

	// Compute Resources required by the Hazelcast container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Persistence configuration
	//+optional
	Persistence *HazelcastPersistenceConfiguration `json:"persistence,omitempty"`

	// B&R Agent configurations
	// +kubebuilder:default:={repository: "docker.io/hazelcast/platform-operator-agent", version: "0.1.29"}
	Agent AgentConfiguration `json:"agent,omitempty"`

	// Jet Engine configuration
	// +kubebuilder:default:={enabled: true, resourceUploadEnabled: false}
	// +optional
	JetEngineConfiguration *JetEngineConfiguration `json:"jet,omitempty"`

	// User Codes to Download into CLASSPATH
	// +optional
	DeprecatedUserCodeDeployment *UserCodeDeploymentConfig `json:"userCodeDeployment,omitempty"`

	// Java Executor Service configurations, see https://docs.hazelcast.com/hazelcast/latest/computing/executor-service
	// +optional
	ExecutorServices []ExecutorServiceConfiguration `json:"executorServices,omitempty"`

	// Durable Executor Service configurations, see https://docs.hazelcast.com/hazelcast/latest/computing/durable-executor-service
	// +optional
	DurableExecutorServices []DurableExecutorServiceConfiguration `json:"durableExecutorServices,omitempty"`

	// Scheduled Executor Service configurations, see https://docs.hazelcast.com/hazelcast/latest/computing/scheduled-executor-service
	// +optional
	ScheduledExecutorServices []ScheduledExecutorServiceConfiguration `json:"scheduledExecutorServices,omitempty"`

	// Hazelcast system properties, see https://docs.hazelcast.com/hazelcast/latest/system-properties
	// +optional
	Properties map[string]string `json:"properties,omitempty"`

	// Logging level for Hazelcast members
	// +kubebuilder:default:="INFO"
	// +optional
	LoggingLevel LoggingLevel `json:"loggingLevel,omitempty"`

	// Configuration to create clusters resilient to node and zone failures
	// +optional
	HighAvailabilityMode HighAvailabilityMode `json:"highAvailabilityMode,omitempty"`

	// Hazelcast JVM configuration
	// +optional
	JVM *JVMConfiguration `json:"jvm,omitempty"`

	// Hazelcast Native Memory (HD Memory) configuration
	// +optional
	NativeMemory *NativeMemoryConfiguration `json:"nativeMemory,omitempty"`

	// Hazelcast Advanced Network configuration
	// +optional
	AdvancedNetwork *AdvancedNetwork `json:"advancedNetwork,omitempty"`

	// Hazelcast Management Center Configuration
	// +optional
	ManagementCenterConfig *ManagementCenterConfig `json:"managementCenter,omitempty"`

	// Hazelcast TLS configuration
	// +optional
	TLS *TLS `json:"tls,omitempty"`

	// Hazelcast serialization configuration
	// +optional
	Serialization *SerializationConfig `json:"serialization,omitempty"`

	// Name of the ConfigMap with the Hazelcast custom configuration.
	// This configuration from the ConfigMap might be overridden by the Hazelcast CR configuration.
	// +optional
	CustomConfigCmName string `json:"customConfigCmName,omitempty"`

	// Hazelcast SQL configuration
	// +optional
	SQL *SQL `json:"sql,omitempty"`

	// Hazelcast LocalDevice configuration
	// +optional
	LocalDevices []LocalDeviceConfig `json:"localDevices,omitempty"`

	// Hazelcast Kubernetes resource annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Hazelcast Kubernetes resource labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// ServiceAccountName is the name of the ServiceAccount to use to run Hazelcast cluster.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// CPSubsystem is the configuration of the Hazelcast CP Subsystem.
	// +optional
	CPSubsystem *CPSubsystem `json:"cpSubsystem,omitempty"`

	// UserCodeNamespaces provide a container for Java classpath resources, such as user code and accompanying artifacts like property files
	// +optional
	UserCodeNamespaces *UserCodeNamespacesConfig `json:"userCodeNamespaces,omitempty"`

	// Env configuration of environment variables
	// +optional
	// +kubebuilder:validation:XValidation:message="Environment variables cannot start with 'HZ_'. Use customConfigCmName to configure Hazelcast.",rule="self.all(env, env.name.startsWith('HZ_') == false)"
	// +kubebuilder:validation:XValidation:message="Environment variable name cannot be empty.",rule="self.all(env, env.name != '')"
	Env []corev1.EnvVar `json:"env,omitempty"`
}

func (s *HazelcastSpec) GetLicenseKeySecretName() string {
	if s.LicenseKeySecretName == "" {
		return s.DeprecatedLicenseKeySecret
	}
	return s.LicenseKeySecretName
}

// +kubebuilder:validation:Enum=Native;BigEndian;LittleEndian
type ByteOrder string

const (
	// NativeByteOrder uses the native byte order of the underlying platform.
	NativeByteOrder ByteOrder = "Native"

	// BigEndian uses the big-endian byte order.
	BigEndian ByteOrder = "BigEndian"

	// LittleEndian uses the kittle-endian byte order.
	LittleEndian ByteOrder = "LittleEndian"
)

func (ucn *UserCodeNamespacesConfig) IsEnabled() bool {
	return ucn != nil
}

type UserCodeNamespacesConfig struct {
	// Blacklist and whitelist for classes when User Code Namespaces is used.
	// +optional
	ClassFilter *JavaFilterConfig `json:"classFilter,omitempty"`
}

// CPSubsystem contains the configuration of a component of a Hazelcast that builds a strongly consistent layer for a set of distributed data structures
type CPSubsystem struct {
	// SessionTTLSeconds is the duration for a CP session to be kept alive after the last received heartbeat.
	// Must be greater than or equal to SessionHeartbeatIntervalSeconds and smaller than or equal to MissingCpMemberAutoRemovalSeconds.
	// +optional
	SessionTTLSeconds *int32 `json:"sessionTTLSeconds,omitempty"`

	// SessionHeartbeatIntervalSeconds Interval in seconds for the periodically committed CP session heartbeats.
	// Must be smaller than SessionTTLSeconds.
	// +optional
	SessionHeartbeatIntervalSeconds *int32 `json:"sessionHeartbeatIntervalSeconds,omitempty"`

	// MissingCpMemberAutoRemovalSeconds is the duration in seconds to wait before automatically removing a missing CP member from the CP Subsystem.
	MissingCpMemberAutoRemovalSeconds *int32 `json:"missingCpMemberAutoRemovalSeconds,omitempty"`

	// FailOnIndeterminateOperationState indicated whether CP Subsystem operations use at-least-once and at-most-once execution guarantees.
	// +optional
	FailOnIndeterminateOperationState *bool `json:"failOnIndeterminateOperationState,omitempty"`

	// DataLoadTimeoutSeconds is the timeout duration in seconds for CP members to restore their persisted data from disk
	// +kubebuilder:validation:Minimum:=1
	// +optional
	DataLoadTimeoutSeconds *int32 `json:"dataLoadTimeoutSeconds,omitempty"`

	// PVC is the configuration of PersistenceVolumeClaim.
	// +optional
	PVC *PvcConfiguration `json:"pvc,omitempty"`
}

// SerializationConfig contains the configuration for the Hazelcast serialization.
type SerializationConfig struct {

	// Specifies the byte order that the serialization will use.
	// +kubebuilder:default:=BigEndian
	// +optional
	ByteOrder ByteOrder `json:"byteOrder"`

	// Allows override of built-in default serializers.
	// +kubebuilder:default:=false
	// +optional
	OverrideDefaultSerializers bool `json:"overrideDefaultSerializers,omitempty"`

	// Enables compression when default Java serialization is used.
	// +kubebuilder:default:=false
	// +optional
	EnableCompression bool `json:"enableCompression,omitempty"`

	// Enables shared object when default Java serialization is used.
	// +kubebuilder:default:=false
	// +optional
	EnableSharedObject bool `json:"enableSharedObject,omitempty"`

	// Allow the usage of unsafe.
	// +kubebuilder:default:=false
	// +optional
	AllowUnsafe bool `json:"allowUnsafe,omitempty"`

	// Lists class implementations of Hazelcast's DataSerializableFactory.
	// +optional
	DataSerializableFactories []string `json:"dataSerializableFactories,omitempty"`

	// Lists class implementations of Hazelcast's PortableFactory.
	// +optional
	PortableFactories []string `json:"portableFactories,omitempty"`

	// List of global serializers.
	// +optional
	GlobalSerializer *GlobalSerializer `json:"globalSerializer,omitempty"`

	// List of serializers (classes) that implemented using Hazelcast's StreamSerializer, ByteArraySerializer etc.
	// +optional
	Serializers []Serializer `json:"serializers,omitempty"`

	// Configuration attributes the compact serialization.
	// +optional
	CompactSerialization *CompactSerializationConfig `json:"compactSerialization,omitempty"`

	// Blacklist and whitelist for deserialized classes when Java serialization is used.
	// +optional
	JavaSerializationFilter *JavaFilterConfig `json:"javaSerializationFilter,omitempty"`
}

// +kubebuilder:validation:MinProperties:=1
type JavaFilterConfig struct {

	// Java deserialization protection Blacklist.
	// +optional
	Blacklist *SerializationFilterList `json:"blacklist,omitempty"`

	// Java deserialization protection Whitelist.
	// +optional
	Whitelist *SerializationFilterList `json:"whitelist,omitempty"`
}

// +kubebuilder:validation:MinProperties:=1
type SerializationFilterList struct {

	// List of class names to be filtered.
	// +optional
	Classes []string `json:"classes,omitempty"`

	// List of packages to be filtered
	// +optional
	Packages []string `json:"packages,omitempty"`

	// List of prefixes to be filtered.
	// +optional
	Prefixes []string `json:"prefixes,omitempty"`
}

// CompactSerializationConfig is the configuration for the Hazelcast Compact serialization.
// +kubebuilder:validation:MinProperties:=1
type CompactSerializationConfig struct {

	// Serializers is the list of explicit serializers to be registered.
	// +optional
	Serializers []string `json:"serializers,omitempty"`

	// Classes is the list of class names for which a zero-config serializer will be registered, without implementing an explicit serializer.
	// +optional
	Classes []string `json:"classes,omitempty"`
}

// Serializer allows to plug in a custom serializer for serializing objects.
type Serializer struct {

	// Name of the class that will be serialized via this implementation.
	// +required
	TypeClass string `json:"typeClass"`

	// Class name of the implementation of the serializer class.
	// +required
	ClassName string `json:"className"`
}

// GlobalSerializer is registered as a fallback serializer to handle all other objects if a serializer cannot be located for them.
type GlobalSerializer struct {

	// If set to true, will replace the internal Java serialization.
	// +optional
	OverrideJavaSerialization *bool `json:"overrideJavaSerialization,omitempty"`

	// Class name of the GlobalSerializer.
	// +required
	ClassName string `json:"className"`
}

type ManagementCenterConfig struct {
	// Allows you to execute scripts that can automate interactions with the cluster.
	// +kubebuilder:default:=false
	// +optional
	ScriptingEnabled bool `json:"scriptingEnabled,omitempty"`

	// Allows you to execute commands from a built-in console in the user interface.
	// +kubebuilder:default:=false
	// +optional
	ConsoleEnabled bool `json:"consoleEnabled,omitempty"`

	// Allows you to access contents of Hazelcast data structures via SQL Browser or Map Browser.
	// +kubebuilder:default:=false
	// +optional
	DataAccessEnabled bool `json:"dataAccessEnabled,omitempty"`
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

	// Configuration for downloading the JAR files be downloaded and accessible for the member.
	// These JAR files will not be placed in the CLASSPATH.
	// +optional
	RemoteFileConfiguration `json:",inline"`

	// Jet Instance Configuration
	// +kubebuilder:default:={}
	// +optional
	Instance *JetInstance `json:"instance,omitempty"`

	// Jet Edge Defaults Configuration
	// +kubebuilder:default:={}
	// +optional
	EdgeDefaults *JetEdgeDefaults `json:"edgeDefaults,omitempty"`
}

type JetInstance struct {
	// The number of threads Jet creates in its cooperative multithreading pool.
	// Its default value is the number of cores
	// +kubebuilder:validation:Minimum:=1
	// +optional
	CooperativeThreadCount *int32 `json:"cooperativeThreadCount,omitempty"`

	// The duration of the interval between flow-control packets.
	// +kubebuilder:default:=100
	// +optional
	FlowControlPeriodMillis int32 `json:"flowControlPeriodMillis,omitempty"`

	// The number of synchronous backups to configure on the IMap that Jet needs internally to store job metadata and snapshots.
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Maximum:=6
	// +optional
	BackupCount int32 `json:"backupCount,omitempty"`

	// The delay after which the auto-scaled jobs restart if a new member joins the cluster.
	// +kubebuilder:default:=10000
	// +optional
	ScaleUpDelayMillis int32 `json:"scaleUpDelayMillis,omitempty"`

	// Specifies whether the Lossless Cluster Restart feature is enabled.
	// +kubebuilder:default:=false
	// +optional
	LosslessRestartEnabled bool `json:"losslessRestartEnabled"`

	// Specifies the maximum number of records that can be accumulated by any single processor instance.
	// Default value is Long.MAX_VALUE
	// +optional
	MaxProcessorAccumulatedRecords *int64 `json:"maxProcessorAccumulatedRecords,omitempty"`
}

// Returns true if Jet Instance section is configured.
func (j *JetInstance) IsConfigured() bool {
	return j != nil && !(*j == (JetInstance{}))
}

type JetEdgeDefaults struct {
	// Sets the capacity of processor-to-processor concurrent queues.
	// +optional
	QueueSize *int32 `json:"queueSize,omitempty"`

	// Limits the size of the packet in bytes.
	// +optional
	PacketSizeLimit *int32 `json:"packetSizeLimit,omitempty"`

	// Sets the scaling factor used by the adaptive receive window sizing function.
	// +optional
	ReceiveWindowMultiplier *int8 `json:"receiveWindowMultiplier,omitempty"`
}

// Returns true if Jet Instance Edge Defaults is configured.
func (j *JetEdgeDefaults) IsConfigured() bool {
	return j != nil && !(*j == (JetEdgeDefaults{}))
}

// Returns true if Jet section is configured.
func (j *JetEngineConfiguration) IsConfigured() bool {
	return j != nil
}

// Returns true if jet is enabled.
func (j *JetEngineConfiguration) IsEnabled() bool {
	return j != nil && j.Enabled != nil && *j.Enabled
}

// Returns true if jet.bucketConfiguration is specified.
func (j *JetEngineConfiguration) IsBucketEnabled() bool {
	return j != nil && j.RemoteFileConfiguration.BucketConfiguration != nil
}

// Returns true if jet.configMaps configuration is specified.
func (j *JetEngineConfiguration) IsConfigMapEnabled() bool {
	return j != nil && (len(j.RemoteFileConfiguration.ConfigMaps) != 0)
}

// Returns true if jet.RemoteURLs configuration is specified.
func (j *JetEngineConfiguration) IsRemoteURLsEnabled() bool {
	return j != nil && (len(j.RemoteFileConfiguration.RemoteURLs) != 0)
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

	// Name of the User Code Namespace applied to this instance
	// +kubebuilder:validation:MinLength:=1
	// +optional
	UserCodeNamespace string `json:"userCodeNamespace,omitempty"`
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

	// Name of the User Code Namespace applied to this instance
	// +kubebuilder:validation:MinLength:=1
	// +optional
	UserCodeNamespace string `json:"userCodeNamespace,omitempty"`
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

	// Name of the User Code Namespace applied to this instance
	// +kubebuilder:validation:MinLength:=1
	// +optional
	UserCodeNamespace string `json:"userCodeNamespace,omitempty"`
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

// UserCodeDeploymentConfig contains the configuration for User Code download operation
type UserCodeDeploymentConfig struct {
	// When true, allows user code deployment from clients.
	// +optional
	ClientEnabled *bool `json:"clientEnabled,omitempty"`

	// A string for triggering a rolling restart for re-downloading the user code.
	// +optional
	TriggerSequence string `json:"triggerSequence,omitempty"`

	// Configuration for files to download. Files downloaded will be put under Java CLASSPATH.
	// +optional
	RemoteFileConfiguration `json:",inline"`
}

type RemoteFileConfiguration struct {
	// Bucket config from where the JAR files will be downloaded.
	// +optional
	BucketConfiguration *BucketConfiguration `json:"bucketConfig,omitempty"`

	// Names of the list of ConfigMaps. Files in each ConfigMap will be downloaded.
	// +optional
	ConfigMaps []string `json:"configMaps,omitempty"`

	// List of URLs from where the files will be downloaded.
	// +optional
	RemoteURLs []string `json:"remoteURLs,omitempty"`
}

type BucketConfiguration struct {
	// secret is a deprecated alias for secretName.
	// +optional
	DeprecatedSecret string `json:"secret"`

	// Name of the secret with credentials for cloud providers.
	// +optional
	SecretName string `json:"secretName"`

	// URL of the bucket to download HotBackup folders.
	// AWS S3, GCP Bucket and Azure Blob storage buckets are supported.
	// Example bucket URIs:
	// - AWS S3     -> s3://bucket-name/path/to/folder
	// - GCP Bucket -> gs://bucket-name/path/to/folder
	// - Azure Blob -> azblob://bucket-name/path/to/folder
	// +kubebuilder:validation:MinLength:=6
	// +required
	BucketURI string `json:"bucketURI"`
}

func (b *BucketConfiguration) GetSecretName() string {
	if b.SecretName == "" {
		return b.DeprecatedSecret
	}
	return b.SecretName
}

// Returns true if userCodeDeployment.bucketConfiguration is specified.
func (c *UserCodeDeploymentConfig) IsBucketEnabled() bool {
	return c != nil && c.RemoteFileConfiguration.BucketConfiguration != nil
}

// Returns true if userCodeDeployment.configMaps configuration is specified.
func (c *UserCodeDeploymentConfig) IsConfigMapEnabled() bool {
	return c != nil && (len(c.RemoteFileConfiguration.ConfigMaps) != 0)
}

// Returns true if userCodeDeployment.RemoteURLs configuration is specified.
func (c *UserCodeDeploymentConfig) IsRemoteURLsEnabled() bool {
	return c != nil && (len(c.RemoteFileConfiguration.RemoteURLs) != 0)
}

type AgentConfiguration struct {
	// Repository to pull Hazelcast Platform Operator Agent(https://github.com/hazelcast/platform-operator-agent)
	// +kubebuilder:default:="docker.io/hazelcast/platform-operator-agent"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version of Hazelcast Platform Operator Agent.
	// +kubebuilder:default:="0.1.29"
	// +optional
	Version string `json:"version,omitempty"`

	// Compute Resources required by the Agent container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// HazelcastPersistenceConfiguration contains the configuration for Hazelcast Persistence and K8s storage.
type HazelcastPersistenceConfiguration struct {
	// BaseDir is deprecated. Use restore.localConfig to restore from a local backup.
	// +optional
	DeprecatedBaseDir string `json:"baseDir"`

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
	// +required
	PVC *PvcConfiguration `json:"pvc,omitempty"`

	// Restore configuration
	// +kubebuilder:default:={}
	// +optional
	Restore RestoreConfiguration `json:"restore,omitempty"`
}

// Returns true if ClusterDataRecoveryPolicy is not FullRecoveryOnly
func (p *HazelcastPersistenceConfiguration) AutoRemoveStaleData() bool {
	return p.ClusterDataRecoveryPolicy != FullRecovery
}

// IsEnabled Returns true if Persistence configuration is specified.
func (p *HazelcastPersistenceConfiguration) IsEnabled() bool {
	return p != nil
}

func (p *CPSubsystem) IsEnabled() bool {
	return p != nil
}

func (p *CPSubsystem) IsPVC() bool {
	return p != nil && p.PVC != nil
}

// IsRestoreEnabled returns true if Restore configuration is specified
func (p *HazelcastPersistenceConfiguration) IsRestoreEnabled() bool {
	return p.IsEnabled() && !(p.Restore == (RestoreConfiguration{}))
}

// RestoreFromHotBackupResourceName returns true if Restore is done from a HotBackup resource
func (p *HazelcastPersistenceConfiguration) RestoreFromHotBackupResourceName() bool {
	return p.IsRestoreEnabled() && p.Restore.HotBackupResourceName != ""
}

// RestoreFromLocalBackup returns true if Restore is done from local backup
func (p *HazelcastPersistenceConfiguration) RestoreFromLocalBackup() bool {
	return p.IsRestoreEnabled() && p.Restore.LocalConfiguration != nil
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

	// Configuration to restore from local backup
	// +optional
	LocalConfiguration *RestoreFromLocalConfiguration `json:"localConfig,omitempty"`
}

// PVCNamePrefix specifies the prefix of existing PVCs
// +kubebuilder:validation:Enum=persistence;hot-restart-persistence
type PVCNamePrefix string

const (
	// Persistence format is persistence.
	Persistence PVCNamePrefix = "persistence"

	// HotRestartPersistence format is hot-restart-persistence.
	HotRestartPersistence PVCNamePrefix = "hot-restart-persistence"
)

type RestoreFromLocalConfiguration struct {
	// PVC name prefix used in existing PVCs
	// +optional
	// +kubebuilder:default:="persistence"
	PVCNamePrefix PVCNamePrefix `json:"pvcNamePrefix,omitempty"`

	// Persistence base directory
	// +optional
	BaseDir string `json:"baseDir,omitempty"`

	// Local backup base directory
	// +optional
	BackupDir string `json:"backupDir,omitempty"`

	// Backup directory
	// +optional
	// +kubebuilder:validation:MinLength:=1
	BackupFolder string `json:"backupFolder,omitempty"`
}

func (rc RestoreConfiguration) Hash() string {
	str, _ := json.Marshal(rc)
	return strconv.Itoa(int(FNV32a(string(str))))
}

type PvcConfiguration struct {
	// AccessModes contains the actual access modes of the volume backing the PVC has.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`

	// A description of the PVC request capacity.
	// +kubebuilder:default:="8Gi"
	// +optional
	RequestStorage *resource.Quantity `json:"requestStorage,omitempty"`

	// Name of StorageClass which this persistent volume belongs to.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
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

// JVMConfiguration is a Hazelcast JVM configuration
type JVMConfiguration struct {
	// Memory is a JVM memory configuration
	// +optional
	Memory *JVMMemoryConfiguration `json:"memory,omitempty"`

	// GC is for configuring JVM Garbage Collector
	// +optional
	GC *JVMGCConfiguration `json:"gc,omitempty"`

	// Args is for arbitrary JVM arguments
	// +optional
	Args []string `json:"args,omitempty"`
}

func (c *JVMConfiguration) GetMemory() *JVMMemoryConfiguration {
	if c != nil {
		return c.Memory
	}
	return nil
}

func (c *JVMConfiguration) GCConfig() *JVMGCConfiguration {
	if c != nil {
		return c.GC
	}
	return nil
}

func (c *JVMConfiguration) GetArgs() []string {
	if c != nil {
		return c.Args
	}
	return nil
}

// JVMMemoryConfiguration is a JVM memory configuration
type JVMMemoryConfiguration struct {
	// InitialRAMPercentage configures JVM initial heap size
	// +optional
	InitialRAMPercentage *string `json:"initialRAMPercentage,omitempty"`

	// MinRAMPercentage sets the minimum heap size for a JVM
	// +optional
	MinRAMPercentage *string `json:"minRAMPercentage,omitempty"`

	// MaxRAMPercentage sets the maximum heap size for a JVM
	// +optional
	MaxRAMPercentage *string `json:"maxRAMPercentage,omitempty"`
}

func (c *JVMMemoryConfiguration) GetInitialRAMPercentage() string {
	if c != nil && c.InitialRAMPercentage != nil {
		return *c.InitialRAMPercentage
	}
	return ""
}

func (c *JVMMemoryConfiguration) GetMinRAMPercentage() string {
	if c != nil && c.MinRAMPercentage != nil {
		return *c.MinRAMPercentage
	}
	return ""
}

func (c *JVMMemoryConfiguration) GetMaxRAMPercentage() string {
	if c != nil && c.MaxRAMPercentage != nil {
		return *c.MaxRAMPercentage
	}
	return ""
}

// JVMGCConfiguration is for configuring JVM Garbage Collector
type JVMGCConfiguration struct {
	// Logging enables logging when set to true
	// +optional
	Logging *bool `json:"logging,omitempty"`

	// Collector is the Garbage Collector type
	// +optional
	Collector *GCType `json:"collector,omitempty"`
}

func (j *JVMGCConfiguration) IsLoggingEnabled() bool {
	if j != nil && j.Logging != nil {
		return *j.Logging
	}
	return false
}

func (j *JVMGCConfiguration) GetCollector() GCType {
	if j != nil && j.Collector != nil {
		return *j.Collector
	}
	return ""
}

// GCType is Garbage Collector type
// +kubebuilder:validation:Enum=Serial;Parallel;G1
type GCType string

const (
	GCTypeSerial   GCType = "Serial"
	GCTypeParallel GCType = "Parallel"
	GCTypeG1       GCType = "G1"
)

// NativeMemoryAllocatorType is one of 2 types of mechanism for allocating HD Memory
// +kubebuilder:validation:Enum=STANDARD;POOLED
type NativeMemoryAllocatorType string

const (
	// NativeMemoryStandard allocate memory using default OS memory manager
	NativeMemoryStandard NativeMemoryAllocatorType = "STANDARD"

	// NativeMemoryPooled is Hazelcast own pooling memory allocator
	NativeMemoryPooled NativeMemoryAllocatorType = "POOLED"
)

// NativeMemoryConfiguration is a Hazelcast HD memory configuration
type NativeMemoryConfiguration struct {
	// AllocatorType specifies one of 2 types of mechanism for allocating memory to HD
	// +kubebuilder:default:="STANDARD"
	// +optional
	AllocatorType NativeMemoryAllocatorType `json:"allocatorType,omitempty"`

	// Size of the total native memory to allocate
	// +kubebuilder:default:="512M"
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// MinBlockSize is the size of smallest block that will be allocated.
	// It is used only by the POOLED memory allocator.
	// +optional
	MinBlockSize int32 `json:"minBlockSize,omitempty"`

	// PageSize is the size of the page in bytes to allocate memory as a block.
	// It is used only by the POOLED memory allocator.
	// +kubebuilder:default:=4194304
	// +optional
	PageSize int32 `json:"pageSize,omitempty"`

	// MetadataSpacePercentage defines percentage of the allocated native memory
	// that is used for the metadata of other map components such as index
	// (for predicates), offset, etc.
	// +kubebuilder:default:=12
	// +optional
	MetadataSpacePercentage int32 `json:"metadataSpacePercentage,omitempty"`
}

func (c *NativeMemoryConfiguration) IsEnabled() bool {
	return c != nil && !(*c == (NativeMemoryConfiguration{}))
}

type AdvancedNetwork struct {
	// +optional
	MemberServerSocketEndpointConfig ServerSocketEndpointConfig `json:"memberServerSocketEndpointConfig,omitempty"`

	// +optional
	ClientServerSocketEndpointConfig ServerSocketEndpointConfig `json:"clientServerSocketEndpointConfig,omitempty"`

	// +optional
	WAN []WANConfig `json:"wan,omitempty"`
}

type WANConfig struct {
	Port        uint               `json:"port,omitempty"`
	PortCount   uint               `json:"portCount,omitempty"`
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
	// +kubebuilder:validation:MaxLength:=8
	Name string `json:"name,omitempty"`
}

type ServerSocketEndpointConfig struct {
	Interfaces []string `json:"interfaces,omitempty"`
}

// +kubebuilder:validation:Enum=None;Required;Optional
type MutualAuthentication string

const (
	// Client side of connection is not authenticated.
	MutualAuthenticationNone MutualAuthentication = "None"

	// Server asks for client certificate. If the client does not provide a
	// keystore or the provided keystore is not verified against member’s
	// truststore, the client is not authenticated.
	MutualAuthenticationRequired MutualAuthentication = "Required"

	// Server asks for client certificate, but client is not required
	// to provide any valid certificate.
	MutualAuthenticationOptional MutualAuthentication = "Optional"
)

type TLS struct {
	// Name of the secret with TLS certificate and key.
	SecretName string `json:"secretName"`

	// Mutual authentication configuration. It’s None by default which
	// means the client side of connection is not authenticated.
	// +kubebuilder:default:="None"
	// +optional
	MutualAuthentication MutualAuthentication `json:"mutualAuthentication,omitempty"`
}

type SQL struct {
	// StatementTimeout defines the timeout in milliseconds that is applied
	// to queries without an explicit timeout.
	// +kubebuilder:default:=0
	// +optional
	StatementTimeout int32 `json:"statementTimeout"`

	// CatalogPersistenceEnabled sets whether SQL Catalog persistence is enabled for the node.
	// With SQL Catalog persistence enabled you can restart the whole cluster without
	// losing schema definition objects (such as MAPPINGs, TYPEs, VIEWs and DATA CONNECTIONs).
	// The feature is implemented on top of the Hot Restart feature of Hazelcast
	// which persists the data to disk. If enabled, you have to also configure
	// Hot Restart. Feature is disabled by default.
	// +kubebuilder:default:=false
	// +optional
	CatalogPersistenceEnabled bool `json:"catalogPersistenceEnabled"`
}

type LocalDeviceConfig struct {
	// Name represents the name of the local device
	// +required
	Name string `json:"name"`

	// BlockSize defines Device block/sector size in bytes.
	// +kubebuilder:validation:Minimum=512
	// +kubebuilder:default:=4096
	// +optional
	BlockSize *int32 `json:"blockSize,omitempty"`

	// ReadIOThreadCount is Read IO thread count.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:default:=4
	// +optional
	ReadIOThreadCount *int32 `json:"readIOThreadCount,omitempty"`

	// WriteIOThreadCount is Write IO thread count.
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:default:=4
	// +optional
	WriteIOThreadCount *int32 `json:"writeIOThreadCount,omitempty"`

	// Configuration of PersistenceVolumeClaim.
	// +required
	PVC *PvcConfiguration `json:"pvc,omitempty"`
}

// IsTieredStorageEnabled Returns true if LocalDevices configuration is specified.
func (hs *HazelcastSpec) IsTieredStorageEnabled() bool {
	return len(hs.LocalDevices) != 0
}

// HazelcastStatus defines the observed state of Hazelcast
type HazelcastStatus struct {
	// Number of Hazelcast members in the cluster.
	// +optional
	ClusterSize int32 `json:"clusterSize"`

	// Selector is a label selector used by HorizontalPodAutoscaler to autoscale Hazelcast resource.
	// +optional
	Selector string `json:"selector"`

	// Phase of the Hazelcast cluster
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Status of the Hazelcast cluster
	// +optional
	Cluster HazelcastClusterStatus `json:"hazelcastClusterStatus,omitempty"`

	// Message about the Hazelcast cluster state
	// +optional
	Message string `json:"message,omitempty"`

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
// +kubebuilder:subresource:scale:specpath=.spec.clusterSize,statuspath=.status.clusterSize,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the Hazelcast deployment"
// +kubebuilder:printcolumn:name="Members",type="string",JSONPath=".status.hazelcastClusterStatus.readyMembers",description="Current numbers of ready Hazelcast members"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current Hazelcast Config"
// +kubebuilder:resource:shortName=hz
type Hazelcast struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

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
