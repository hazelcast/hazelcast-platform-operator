package config

type HazelcastWrapper struct {
	Hazelcast Hazelcast `yaml:"hazelcast"`
}

type Hazelcast struct {
	Jet                      Jet                                 `yaml:"jet,omitempty"`
	ClusterName              string                              `yaml:"cluster-name,omitempty"`
	Persistence              Persistence                         `yaml:"persistence,omitempty"`
	Map                      map[string]Map                      `yaml:"map,omitempty"`
	ExecutorService          map[string]ExecutorService          `yaml:"executor-service,omitempty"`
	DurableExecutorService   map[string]DurableExecutorService   `yaml:"durable-executor-service,omitempty"`
	ScheduledExecutorService map[string]ScheduledExecutorService `yaml:"scheduled-executor-service,omitempty"`
	UserCodeDeployment       UserCodeDeployment                  `yaml:"user-code-deployment,omitempty"`
	WanReplication           map[string]WanReplicationConfig     `yaml:"wan-replication,omitempty"`
	Properties               map[string]string                   `yaml:"properties,omitempty"`
	MultiMap                 map[string]MultiMap                 `yaml:"multimap,omitempty"`
	Topic                    map[string]Topic                    `yaml:"topic,omitempty"`
	ReplicatedMap            map[string]ReplicatedMap            `yaml:"replicatedmap,omitempty"`
	Queue                    map[string]Queue                    `yaml:"queue,omitempty"`
	Cache                    map[string]Cache                    `yaml:"cache,omitempty"`
	PartitionGroup           PartitionGroup                      `yaml:"partition-group,omitempty"`
	NativeMemory             NativeMemory                        `yaml:"native-memory,omitempty"`
	AdvancedNetwork          AdvancedNetwork                     `yaml:"advanced-network,omitempty"`
	ManagementCenter         ManagementCenterConfig              `yaml:"management-center,omitempty"`
	Serialization            Serialization                       `yaml:"serialization,omitempty"`
	SQL                      SQL                                 `yaml:"sql,omitempty"`
	LocalDevice              map[string]LocalDevice              `yaml:"local-device,omitempty"`
}

type ManagementCenterConfig struct {
	ScriptingEnabled  bool `yaml:"scripting-enabled,omitempty"`
	ConsoleEnabled    bool `yaml:"console-enabled,omitempty"`
	DataAccessEnabled bool `yaml:"data-access-enabled,omitempty"`
}

type AdvancedNetwork struct {
	Enabled                          bool                             `yaml:"enabled,omitempty"`
	Join                             Join                             `yaml:"join,omitempty"`
	MemberServerSocketEndpointConfig MemberServerSocketEndpointConfig `yaml:"member-server-socket-endpoint-config,omitempty"`
	ClientServerSocketEndpointConfig ClientServerSocketEndpointConfig `yaml:"client-server-socket-endpoint-config,omitempty"`
	RestServerSocketEndpointConfig   RestServerSocketEndpointConfig   `yaml:"rest-server-socket-endpoint-config,omitempty"`
	WanServerSocketEndpointConfig    map[string]WanPort               `yaml:"wan-server-socket-endpoint-config,omitempty"`
}

type MemberServerSocketEndpointConfig struct {
	Port       PortAndPortCount     `yaml:"port,omitempty"`
	Interfaces EnabledAndInterfaces `yaml:"interfaces,omitempty"`
	SSL        SSL                  `yaml:"ssl,omitempty"`
}

type ClientServerSocketEndpointConfig struct {
	Port PortAndPortCount `yaml:"port,omitempty"`
	SSL  SSL              `yaml:"ssl,omitempty"`
}

type RestServerSocketEndpointConfig struct {
	Port           PortAndPortCount `yaml:"port,omitempty"`
	EndpointGroups EndpointGroups   `yaml:"endpoint-groups,omitempty"`
	SSL            SSL              `yaml:"ssl,omitempty"`
}

type WanPort struct {
	PortAndPortCount PortAndPortCount `yaml:"port,omitempty"`
}

type PortAndPortCount struct {
	Port      uint `yaml:"port,omitempty"`
	PortCount uint `yaml:"port-count,omitempty"`
}

type EnabledAndInterfaces struct {
	Enabled    bool     `yaml:"enabled,omitempty"`
	Interfaces []string `yaml:"interfaces,omitempty"`
}

type PartitionGroup struct {
	Enabled   *bool  `yaml:"enabled,omitempty"`
	GroupType string `yaml:"group-type,omitempty"`
}

type Jet struct {
	Enabled               *bool        `yaml:"enabled,omitempty"`
	ResourceUploadEnabled *bool        `yaml:"resource-upload-enabled,omitempty"`
	Instance              JetInstance  `yaml:"instance,omitempty"`
	EdgeDefaults          EdgeDefaults `yaml:"edge-defaults,omitempty"`
}

type JetInstance struct {
	CooperativeThreadCount         *int32 `yaml:"cooperative-thread-count,omitempty"`
	FlowControlPeriodMillis        *int32 `yaml:"flow-control-period,omitempty"`
	BackupCount                    *int32 `yaml:"backup-count,omitempty"`
	ScaleUpDelayMillis             *int32 `yaml:"scale-up-delay-millis,omitempty"`
	LosslessRestartEnabled         *bool  `yaml:"lossless-restart-enabled,omitempty"`
	MaxProcessorAccumulatedRecords *int64 `yaml:"max-processor-accumulated-records,omitempty"`
}

type EdgeDefaults struct {
	QueueSize               *int32 `yaml:"queue-size,omitempty"`
	PacketSizeLimit         *int32 `yaml:"packet-size-limit,omitempty"`
	ReceiveWindowMultiplier *int8  `yaml:"receive-window-multiplier,omitempty"`
}

type Network struct {
	Join    Join    `yaml:"join,omitempty"`
	RestAPI RestAPI `yaml:"rest-api,omitempty"`
}

type Join struct {
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`
}

type Persistence struct {
	Enabled                   *bool  `yaml:"enabled,omitempty"`
	BaseDir                   string `yaml:"base-dir"`
	BackupDir                 string `yaml:"backup-dir,omitempty"`
	Parallelism               int32  `yaml:"parallelism"`
	ValidationTimeoutSec      int32  `yaml:"validation-timeout-seconds"`
	ClusterDataRecoveryPolicy string `yaml:"cluster-data-recovery-policy"`
	AutoRemoveStaleData       *bool  `yaml:"auto-remove-stale-data"`
}

type Kubernetes struct {
	Enabled                      *bool  `yaml:"enabled,omitempty"`
	Namespace                    string `yaml:"namespace,omitempty"`
	ServiceName                  string `yaml:"service-name,omitempty"`
	UseNodeNameAsExternalAddress *bool  `yaml:"use-node-name-as-external-address,omitempty"`
	ServicePerPodLabelName       string `yaml:"service-per-pod-label-name,omitempty"`
	ServicePerPodLabelValue      string `yaml:"service-per-pod-label-value,omitempty"`
	ServicePort                  uint   `yaml:"service-port,omitempty"`
}

type RestAPI struct {
	Enabled        *bool          `yaml:"enabled,omitempty"`
	EndpointGroups EndpointGroups `yaml:"endpoint-groups,omitempty"`
}

type EndpointGroups struct {
	HealthCheck  EndpointGroup `yaml:"HEALTH_CHECK,omitempty"`
	ClusterWrite EndpointGroup `yaml:"CLUSTER_WRITE,omitempty"`
	Persistence  EndpointGroup `yaml:"PERSISTENCE,omitempty"`
}

type EndpointGroup struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

type Map struct {
	BackupCount             int32                              `yaml:"backup-count"`
	AsyncBackupCount        int32                              `yaml:"async-backup-count"`
	TimeToLiveSeconds       int32                              `yaml:"time-to-live-seconds"`
	MaxIdleSeconds          int32                              `yaml:"max-idle-seconds"`
	Eviction                MapEviction                        `yaml:"eviction,omitempty"`
	ReadBackupData          bool                               `yaml:"read-backup-data"`
	InMemoryFormat          string                             `yaml:"in-memory-format"`
	StatisticsEnabled       bool                               `yaml:"statistics-enabled"`
	Indexes                 []MapIndex                         `yaml:"indexes,omitempty"`
	DataPersistence         DataPersistence                    `yaml:"data-persistence,omitempty"`
	WanReplicationReference map[string]WanReplicationReference `yaml:"wan-replication-ref,omitempty"`
	MapStoreConfig          MapStoreConfig                     `yaml:"map-store,omitempty"`
	EntryListeners          []EntryListener                    `yaml:"entry-listeners,omitempty"`
	NearCache               NearCacheConfig                    `yaml:"near-cache,omitempty"`
	EventJournal            EventJournal                       `yaml:"event-journal,omitempty"`
	TieredStore             TieredStore                        `yaml:"tiered-store,omitempty"`
}

type EntryListener struct {
	ClassName    string `yaml:"class-name"`
	IncludeValue bool   `yaml:"include-value"`
	Local        bool   `yaml:"local"`
}

type MapEviction struct {
	Size           int32  `yaml:"size"`
	MaxSizePolicy  string `yaml:"max-size-policy,omitempty"`
	EvictionPolicy string `yaml:"eviction-policy,omitempty"`
}

type MapIndex struct {
	Name               string             `yaml:"name,omitempty"`
	Type               string             `yaml:"type"`
	Attributes         []string           `yaml:"attributes"`
	BitmapIndexOptions BitmapIndexOptions `yaml:"bitmap-index-options,omitempty"`
}

type BitmapIndexOptions struct {
	UniqueKey               string `yaml:"unique-key"`
	UniqueKeyTransformation string `yaml:"unique-key-transformation"`
}

type DataPersistence struct {
	Enabled bool `yaml:"enabled"`
	Fsync   bool `yaml:"fsync"`
}

type WanReplicationReference struct {
	MergePolicyClassName string   `yaml:"merge-policy-class-name"`
	RepublishingEnabled  bool     `yaml:"republishing-enabled"`
	Filters              []string `yaml:"filters"`
}

type ExecutorService struct {
	PoolSize      int32 `yaml:"pool-size"`
	QueueCapacity int32 `yaml:"queue-capacity"`
}

type DurableExecutorService struct {
	PoolSize   int32 `yaml:"pool-size"`
	Durability int32 `yaml:"durability"`
	Capacity   int32 `yaml:"capacity"`
}

type ScheduledExecutorService struct {
	PoolSize       int32  `yaml:"pool-size"`
	Durability     int32  `yaml:"durability"`
	Capacity       int32  `yaml:"capacity"`
	CapacityPolicy string `yaml:"capacity-policy"`
}

type MapStoreConfig struct {
	Enabled           bool              `yaml:"enabled"`
	WriteCoalescing   *bool             `yaml:"write-coalescing,omitempty"`
	WriteDelaySeconds int32             `yaml:"write-delay-seconds"`
	WriteBatchSize    int32             `yaml:"write-batch-size"`
	ClassName         string            `yaml:"class-name"`
	Properties        map[string]string `yaml:"properties,omitempty"`
	InitialLoadMode   string            `yaml:"initial-mode"`
}

type NearCacheConfig struct {
	InMemoryFormat     string            `yaml:"in-memory-format"`
	InvalidateOnChange bool              `yaml:"invalidate-on-change"`
	TimeToLiveSeconds  uint              `yaml:"time-to-live-seconds"`
	MaxIdleSeconds     uint              `yaml:"max-idle-seconds"`
	CacheLocalEntries  bool              `yaml:"cache-local-entries"`
	Eviction           NearCacheEviction `yaml:"eviction"`
}

type NearCacheEviction struct {
	Size           uint32 `yaml:"size"`
	MaxSizePolicy  string `yaml:"max-size-policy,omitempty"`
	EvictionPolicy string `yaml:"eviction-policy,omitempty"`
}

type EventJournal struct {
	Enabled           bool  `json:"enabled"`
	Capacity          int32 `json:"capacity"`
	TimeToLiveSeconds int32 `json:"time-to-live-seconds"`
}

type TieredStore struct {
	Enabled    bool       `json:"enabled"`
	MemoryTier MemoryTier `json:"memory-tier"`
	DiskTier   DiskTier   `json:"disk-tier"`
}

type MemoryTier struct {
	Capacity Size `yaml:"capacity,omitempty"`
}

type DiskTier struct {
	Enabled    bool   `json:"enabled"`
	DeviceName string `json:"device-name"`
}

type Topic struct {
	GlobalOrderingEnabled bool     `yaml:"global-ordering-enabled"`
	MultiThreadingEnabled bool     `yaml:"multi-threading-enabled"`
	StatisticsEnabled     bool     `yaml:"statistics-enabled"`
	MessageListeners      []string `yaml:"message-listeners,omitempty"`
}

type UserCodeDeployment struct {
	Enabled           *bool  `yaml:"enabled,omitempty"`
	ClassCacheMode    string `yaml:"class-cache-mode,omitempty"`
	ProviderMode      string `yaml:"provider-mode,omitempty"`
	BlacklistPrefixes string `yaml:"blacklist-prefixes,omitempty"`
	WhitelistPrefixes string `yaml:"whitelist-prefixes,omitempty"`
	ProviderFilter    string `yaml:"provider-filter,omitempty"`
}

type MultiMap struct {
	BackupCount       int32       `yaml:"backup-count"`
	AsyncBackupCount  int32       `yaml:"async-backup-count"`
	Binary            bool        `yaml:"binary"`
	CollectionType    string      `yaml:"value-collection-type"`
	StatisticsEnabled bool        `yaml:"statistics-enabled"`
	MergePolicy       MergePolicy `yaml:"merge-policy"`
}

type Queue struct {
	BackupCount             int32       `yaml:"backup-count"`
	AsyncBackupCount        int32       `yaml:"async-backup-count"`
	EmptyQueueTtl           int32       `yaml:"empty-queue-ttl"`
	MaxSize                 int32       `yaml:"max-size"`
	StatisticsEnabled       bool        `yaml:"statistics-enabled"`
	MergePolicy             MergePolicy `yaml:"merge-policy"`
	PriorityComparatorClass string      `yaml:"priority-comparator-class-name"`
}

type Cache struct {
	BackupCount       int32           `yaml:"backup-count"`
	AsyncBackupCount  int32           `yaml:"async-backup-count"`
	StatisticsEnabled bool            `yaml:"statistics-enabled"`
	ManagementEnabled bool            `yaml:"management-enabled"`
	ReadThrough       bool            `yaml:"read-through"`
	WriteThrough      bool            `yaml:"write-through"`
	MergePolicy       MergePolicy     `yaml:"merge-policy"`
	KeyType           ClassType       `yaml:"key-type,omitempty"`
	ValueType         ClassType       `yaml:"value-type,omitempty"`
	InMemoryFormat    string          `yaml:"in-memory-format"`
	DataPersistence   DataPersistence `yaml:"data-persistence,omitempty"`
	EventJournal      EventJournal    `yaml:"event-journal,omitempty"`
}

type ClassType struct {
	ClassName string `yaml:"class-name"`
}

type MergePolicy struct {
	ClassName string `yaml:"class-name"`
	BatchSize int32  `yaml:"batch-size"`
}

type ReplicatedMap struct {
	InMemoryFormat    string      `yaml:"in-memory-format"`
	AsyncFillup       bool        `yaml:"async-fillup"`
	StatisticsEnabled bool        `yaml:"statistics-enabled"`
	MergePolicy       MergePolicy `yaml:"merge-policy"`
}

type WanReplicationConfig struct {
	BatchPublisher map[string]BatchPublisherConfig `yaml:"batch-publisher,omitempty"`
}

type BatchPublisherConfig struct {
	ClusterName           string `yaml:"cluster-name,omitempty"`
	BatchSize             int32  `yaml:"batch-size,omitempty"`
	BatchMaxDelayMillis   int32  `yaml:"batch-max-delay-millis,omitempty"`
	ResponseTimeoutMillis int32  `yaml:"response-timeout-millis,omitempty"`
	AcknowledgementType   string `yaml:"acknowledge-type,omitempty"`
	InitialPublisherState string `yaml:"initial-publisher-state,omitempty"`
	QueueFullBehavior     string `yaml:"queue-full-behavior,omitempty"`
	QueueCapacity         int32  `yaml:"queue-capacity,omitempty"`
	TargetEndpoints       string `yaml:"target-endpoints,omitempty"`
}

type NativeMemory struct {
	Enabled                 bool   `yaml:"enabled"`
	AllocatorType           string `yaml:"allocator-type"`
	Size                    Size   `yaml:"size,omitempty"`
	MinBlockSize            int32  `yaml:"min-block-size,omitempty"`
	PageSize                int32  `yaml:"page-size,omitempty"`
	MetadataSpacePercentage int32  `yaml:"metadata-space-percentage,omitempty"`
}

type Size struct {
	Value int64  `yaml:"value"`
	Unit  string `yaml:"unit"`
}

type SSL struct {
	Enabled          *bool         `yaml:"enabled,omitempty"`
	FactoryClassName string        `yaml:"factory-class-name,omitempty"`
	Properties       SSLProperties `yaml:"properties,omitempty"`
}

type SSLProperties struct {
	Protocol             string `yaml:"protocol,omitempty"`
	MutualAuthentication string `yaml:"mutualAuthentication,omitempty"`
	KeyStore             string `yaml:"keyStore,omitempty"`
	KeyStorePassword     string `yaml:"keyStorePassword,omitempty"`
	KeyStoreType         string `yaml:"keyStoreType,omitempty"`
	TrustStore           string `yaml:"trustStore,omitempty"`
	TrustStorePassword   string `yaml:"trustStorePassword,omitempty"`
	TrustStoreType       string `yaml:"trustStoreType,omitempty"`
}

type Serialization struct {
	PortableVersion            int32                    `yaml:"portable-version"`
	UseNativeByteOrder         bool                     `yaml:"use-native-byte-order"`
	EnableCompression          bool                     `yaml:"enable-compression"`
	EnableSharedObject         bool                     `yaml:"enable-shared-object"`
	OverrideDefaultSerializers bool                     `yaml:"allow-override-default-serializers"`
	AllowUnsafe                bool                     `yaml:"allow-unsafe"`
	ByteOrder                  string                   `yaml:"byte-order"`
	DataSerializableFactories  []ClassFactories         `yaml:"data-serializable-factories,omitempty"`
	PortableFactories          []ClassFactories         `yaml:"portable-factories,omitempty"`
	GlobalSerializer           *GlobalSerializer        `yaml:"global-serializer,omitempty"`
	Serializers                []Serializer             `yaml:"serializers,omitempty"`
	JavaSerializationFilter    *JavaSerializationFilter `yaml:"java-serialization-filter,omitempty"`
	CompactSerialization       *CompactSerialization    `yaml:"compact-serialization,omitempty"`
}

type CompactSerialization struct {
	Serializers []string `yaml:"serializers,omitempty"`
	Classes     []string `yaml:"classes,omitempty"`
}

type JavaSerializationFilter struct {
	DefaultsDisable bool        `yaml:"defaults-disabled"`
	Blacklist       *FilterList `yaml:"blacklist,omitempty"`
	Whitelist       *FilterList `yaml:"whitelist,omitempty"`
}

type FilterList struct {
	Classes  []string `yaml:"classes,omitempty"`
	Packages []string `yaml:"packages,omitempty"`
	Prefixes []string `yaml:"prefixes,omitempty"`
}

type Serializer struct {
	TypeClass string `yaml:"type-class"`
	ClassName string `yaml:"class-name"`
}

type GlobalSerializer struct {
	OverrideJavaSerialization *bool  `yaml:"override-java-serialization,omitempty"`
	ClassName                 string `yaml:"class-name"`
}

type ClassFactories struct {
	FactoryId int32  `yaml:"factory-id"`
	ClassName string `yaml:"class-name"`
}

type SQL struct {
	StatementTimeout   int32 `yaml:"statement-timeout-millis"`
	CatalogPersistence bool  `yaml:"catalog-persistence-enabled"`
}

type LocalDevice struct {
	BaseDir            string `yaml:"base-dir"`
	Capacity           Size   `yaml:"capacity,omitempty"`
	BlockSize          *int32 `yaml:"block-size,omitempty"`
	ReadIOThreadCount  *int32 `yaml:"read-io-thread-count"`
	WriteIOThreadCount *int32 `yaml:"write-io-thread-count"`
}
