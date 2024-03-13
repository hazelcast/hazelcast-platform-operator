package naming

import (
	corev1 "k8s.io/api/core/v1"
)

// Labels and label values
const (
	// Finalizer name used by operator
	Finalizer = "hazelcast.com/finalizer"
	// Finalizer name used by operator to stop wan replication of a map
	WanRepMapFinalizer = "hazelcast.com/wan-replicated-map"
	// LicenseDataKey is a key used in k8s secret that holds the Hazelcast license
	LicenseDataKey = "license-key"
	// LicenseKeySecret default license key secret
	LicenseKeySecret = "hazelcast-license-key"
	// ServicePerPodLabelName set to true when the service is a Service per pod
	ServicePerPodLabelName                       = "hazelcast.com/service-per-pod"
	ServicePerPodCountAnnotation                 = "hazelcast.com/service-per-pod-count"
	ExposeExternallyAnnotation                   = "hazelcast.com/expose-externally-member-access"
	LastAppliedSpecAnnotation                    = "hazelcast.com/last-applied-spec"
	LastSuccessfulSpecAnnotation                 = "hazelcast.com/last-successful-spec"
	CurrentHazelcastConfigForcingRestartChecksum = "hazelcast.com/current-hazelcast-config-forcing-restart-checksum"

	// ServiceEndpointTypeLabelName defines the name of the service referred by the HazelcastEndpoint
	ServiceEndpointTypeLabelName           = "hazelcast.com/hazelcast-service-endpoint-type"
	ServiceEndpointTypeDiscoveryLabelValue = "discovery"
	ServiceEndpointTypeMemberLabelValue    = "member"
	ServiceEndpointTypeWANLabelValue       = "wan"

	// PodNameLabel label that represents the name of the pod in the StatefulSet
	PodNameLabel = "statefulset.kubernetes.io/pod-name"
	// ApplicationNameLabel label for the name of the application
	ApplicationNameLabel = "app.kubernetes.io/name"
	// ApplicationInstanceNameLabel label for a unique name identifying the instance of an application
	ApplicationInstanceNameLabel = "app.kubernetes.io/instance"
	// ApplicationManagedByLabel label for the tool being used to manage the operation of an application
	ApplicationManagedByLabel = "app.kubernetes.io/managed-by"

	LabelValueTrue  = "true"
	LabelValueFalse = "false"

	OperatorName         = "hazelcast-platform-operator"
	Hazelcast            = "hazelcast"
	HazelcastPortName    = "hazelcast-port"
	HazelcastStorageName = Hazelcast + "-storage"
	HazelcastMountPath   = "/data/hazelcast"
	TmpDirVolName        = "tmp-vol"

	// ManagementCenter MC name
	ManagementCenter = "management-center"
	// Mancenter MC short name
	Mancenter = "mancenter"
	// MancenterPort MC short name
	MancenterPort = Mancenter + "-port"
	// MancenterStorageName storage name for MC
	MancenterStorageName = Mancenter + "-storage"

	// PersistenceVolumeName is the name the Persistence Volume Claim used in Persistence configuration.
	PersistenceVolumeName       = "hot-restart-persistence"
	CPPersistenceVolumeName     = "cp-subsystem-persistence"
	UserCodeBucketVolumeName    = "user-code-bucket"
	JetJobJarsVolumeName        = "jet-job-jars-bucket"
	JetConfigMapNamePrefix      = "jet-cm-"
	UserCodeURLVolumeName       = "user-code-url"
	UserCodeConfigMapNamePrefix = "user-code-cm-"
	PersistenceMountPath        = "/data/persistence"
	BaseDir                     = PersistenceMountPath + "/base-dir"
	CPDirSuffix                 = "/cp-data"
	CPBaseDir                   = "/data" + CPDirSuffix

	SidecarAgent        = "sidecar-agent"
	BackupAgentPortName = "backup-agent-port"
	RestoreAgent        = "restore-agent"
	RestoreLocalAgent   = "restore-local-agent"
	BucketSecret        = "br-secret"
	UserCodeBucketAgent = "ucd-bucket-agent"
	UserCodeURLAgent    = "ucd-url-agent"
	JetBucketAgent      = "jet-bucket-agent"
	JetUrlAgent         = "jet-url-agent"

	MTLSCertSecretName = "hazelcast-agent-cert"
	MTLSCertPath       = "/var/run/secrets/hazelcast"

	UserCodeBucketPath    = "/opt/hazelcast/userCode/bucket"
	UserCodeURLPath       = "/opt/hazelcast/userCode/urls"
	UserCodeConfigMapPath = "/opt/hazelcast/userCode/cm"
	JetJobJarsPath        = "/opt/hazelcast/jetJobJars"
)

// Annotations
const HazelcastCustomConfigOverwrite = "hazelcast.com/custom-config-overwrite"

// Hazelcast default configurations
const (
	// DefaultHzPort Hazelcast default port
	DefaultHzPort = 5701
	// DefaultClusterSize default number of members of Hazelcast cluster
	DefaultClusterSize = 3
	// DefaultClusterName default name of Hazelcast cluster
	DefaultClusterName = "dev"
	// HazelcastRepo image repository for Hazelcast
	HazelcastRepo = "docker.io/hazelcast/hazelcast"
	// HazelcastEERepo image repository for Hazelcast EE
	HazelcastEERepo = "docker.io/hazelcast/hazelcast-enterprise"
	// HazelcastVersion version of Hazelcast image
	HazelcastVersion = "5.3.5"
	// HazelcastImagePullPolicy pull policy for Hazelcast Platform image
	HazelcastImagePullPolicy = corev1.PullIfNotPresent
)

// Management Center default configurations
const (
	// MCRepo image repository for Management Center
	MCRepo = "docker.io/hazelcast/management-center"
	// MCVersion version of Management Center image
	MCVersion = "5.3.3"
	// MCImagePullPolicy pull policy for Management Center image
	MCImagePullPolicy = corev1.PullIfNotPresent
)

// Map Config default values
const (
	DefaultMapBackupCount        = int32(1)
	DefaultMapAsyncBackupCount   = int32(0)
	DefaultMapTimeToLiveSeconds  = int32(0)
	DefaultMapMaxIdleSeconds     = int32(0)
	DefaultMapPersistenceEnabled = false
	DefaultMapEvictionPolicy     = "NONE"
	DefaultMapMaxSizePolicy      = "PER_NODE"
	DefaultMapMaxSize            = int32(0)
)

// MultiMap Config default values
const (
	DefaultMultiMapBackupCount       = int32(1)
	DefaultMultiMapAsyncBackupCount  = int32(0)
	DefaultMultiMapBinary            = false
	DefaultMultiMapCollectionType    = "SET"
	DefaultMultiMapStatisticsEnabled = true
	DefaultMultiMapMergePolicy       = "com.hazelcast.spi.merge.PutIfAbsentMergePolicy"
	DefaultMultiMapMergeBatchSize    = int32(100)
)

// Queue Config default values
const (
	DefaultQueueBackupCount       = int32(1)
	DefaultQueueAsyncBackupCount  = int32(0)
	DefaultQueueMaxSize           = int32(0)
	DefaultQueueEmptyQueueTtl     = int32(-1)
	DefaultQueueStatisticsEnabled = true
	DefaultQueueMergePolicy       = "com.hazelcast.spi.merge.PutIfAbsentMergePolicy"
	DefaultQueueMergeBatchSize    = int32(100)
)

// Cache Config default values
const (
	DefaultCacheBackupCount                       = int32(1)
	DefaultCacheAsyncBackupCount                  = int32(0)
	DefaultCacheStatisticsEnabled                 = false
	DefaultCacheManagementEnabled                 = false
	DefaultCacheMergePolicy                       = "com.hazelcast.spi.merge.PutIfAbsentMergePolicy"
	DefaultCacheMergeBatchSize                    = int32(100)
	DefaultCacheReadThrough                       = false
	DefaultCacheWriteThrough                      = false
	DefaultCacheInMemoryFormat                    = "BINARY"
	DefaultCacheDisablePerEntryInvalidationEvents = false
)

// ReplicatedMap Config default values
const (
	DefaultReplicatedMapInMemoryFormat    = "OBJECT"
	DefaultReplicatedMapAsyncFillup       = true
	DefaultReplicatedMapStatisticsEnabled = true
	DefaultReplicatedMapMergePolicy       = "com.hazelcast.spi.merge.PutIfAbsentMergePolicy"
	DefaultReplicatedMapMergeBatchSize    = int32(100)
)

// CronHotBackup Config default values
const (
	DefaultSuccessfulHotBackupsHistoryLimit = int32(5)
	DefaultFailedHotBackupsHistoryLimit     = int32(3)
)

// Topic Config default values
const (
	DefaultTopicGlobalOrderingEnabled = false
	DefaultTopicMultiThreadingEnabled = false
	DefaultTopicStatisticsEnabled     = true
)

// Operator Values
const (
	PhoneHomeEnabledEnv              = "PHONE_HOME_ENABLED"
	DeveloperModeEnabledEnv          = "DEVELOPER_MODE_ENABLED"
	PardotIDEnv                      = "PARDOT_ID"
	OperatorVersionEnv               = "OPERATOR_VERSION"
	NamespaceEnv                     = "NAMESPACE"
	WatchedNamespacesEnv             = "WATCHED_NAMESPACES"
	PodNameEnv                       = "POD_NAME"
	HazelcastNodeDiscoveryEnabledEnv = "HAZELCAST_NODE_DISCOVERY_ENABLED"
)

// Backup&Restore agent default configurations
const (
	// DefaultAgentPort Backup&Restore agent default port
	DefaultAgentPort = 8080
)

// WAN related configuration constants
const (
	// DefaultMergePolicyClassName is the default value for
	// merge policy in WAN reference config
	DefaultMergePolicyClassName = "com.hazelcast.spi.merge.PassThroughMergePolicy"
)

// Cluster Size Limit constants
const (
	ClusterSizeLimit = 300
)

const (
	WebhookServerPath = "/tmp/k8s-webhook-server/serving-certs"
)

// Advanced Network Constants
const (
	MemberPortName     = "member-port"
	ClientPortName     = "client-port"
	RestPortName       = "rest-port"
	WanDefaultPortName = "wan-port"
	WanPortNamePrefix  = "wan-"

	MemberServerSocketPort = 5702
	ClientServerSocketPort = 5701
	RestServerSocketPort   = 8081
	WanDefaultPort         = 5710
)

// Jet Engine Config Constants
const (
	MaxBackupCount = 6
)
