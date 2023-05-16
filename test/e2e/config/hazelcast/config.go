package hazelcast

import (
	"flag"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var (
	hazelcastVersion = flag.String("hazelcast-version", naming.HazelcastVersion, "Default Hazelcast version used in e2e tests")
	hazelcastEERepo  = flag.String("hazelcast-ee-repo", naming.HazelcastEERepo, "Enterprise Hazelcast repository used in e2e tests")
	hazelcastRepo    = flag.String("hazelcast-os-repo", naming.HazelcastRepo, "Hazelcast repository used in e2e tests")
)

var (
	ClusterName = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				ClusterName:          "development",
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
			},
		}
	}

	Default = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
			},
		}
	}

	ExposeExternallySmartLoadBalancer = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
				},
			},
		}
	}

	ExposeExternallySmartNodePort = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
				},
			},
		}
	}

	ExposeExternallySmartNodePortNodeName = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortNodeName,
				},
			},
		}
	}

	ExposeExternallyUnisocket = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
				LicenseKeySecretName: licenseKey(ee),
				ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	HazelcastPersistencePVC = func(lk types.NamespacedName, clusterSize int32, labels map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{clusterSize}[0],
				Repository:           repo(true),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(true),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
				Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   "/data/hot-restart",
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					},
				},
			},
		}
	}

	HazelcastRestore = func(hz *hazelcastv1alpha1.Hazelcast, restoreConfig hazelcastv1alpha1.RestoreConfiguration) *hazelcastv1alpha1.Hazelcast {
		hzRestore := &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      hz.Name,
				Namespace: hz.Namespace,
				Labels:    hz.Labels,
			},
			Spec: hz.Spec,
		}
		hzRestore.Spec.Persistence.Restore = restoreConfig
		return hzRestore
	}

	UserCodeBucket = func(lk types.NamespacedName, ee bool, s, bkt string, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{1}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				UserCodeDeployment: hazelcastv1alpha1.UserCodeDeploymentConfig{
					RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
				},
			},
		}
	}

	JetConfigured = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: hazelcastv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
				},
			},
		}
	}

	JetWithBucketConfigured = func(lk types.NamespacedName, ee bool, s, bkt string, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: hazelcastv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
				},
			},
		}
	}

	JetWithUrlConfigured = func(lk types.NamespacedName, ee bool, url string, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: hazelcastv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
						RemoteURLs: []string{url},
					},
				},
			},
		}
	}

	UserCodeURL = func(lk types.NamespacedName, ee bool, urls []string, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{1}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				UserCodeDeployment: hazelcastv1alpha1.UserCodeDeploymentConfig{
					RemoteFileConfiguration: hazelcastv1alpha1.RemoteFileConfiguration{
						RemoteURLs: urls,
					},
				},
			},
		}
	}

	ExecutorService = func(lk types.NamespacedName, ee bool, allExecutorServices map[string]interface{}, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				LoggingLevel:              hazelcastv1alpha1.LoggingLevelDebug,
				ClusterSize:               &[]int32{1}[0],
				Repository:                repo(ee),
				Version:                   *hazelcastVersion,
				LicenseKeySecretName:      licenseKey(ee),
				ExecutorServices:          allExecutorServices["es"].([]hazelcastv1alpha1.ExecutorServiceConfiguration),
				DurableExecutorServices:   allExecutorServices["des"].([]hazelcastv1alpha1.DurableExecutorServiceConfiguration),
				ScheduledExecutorServices: allExecutorServices["ses"].([]hazelcastv1alpha1.ScheduledExecutorServiceConfiguration),
			},
		}
	}

	HighAvailability = func(lk types.NamespacedName, ee bool, size int32, mode hazelcastv1alpha1.HighAvailabilityMode, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &size,
				HighAvailabilityMode: mode,
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	HazelcastTLS = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				TLS: hazelcastv1alpha1.TLS{
					SecretName: lk.Name + "-tls",
				},
			},
		}
	}

	HotBackupBucket = func(lk types.NamespacedName, hzName string, lbls map[string]string, bucketURI, secretName string) *hazelcastv1alpha1.HotBackup {
		return &hazelcastv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
				BucketURI:             bucketURI,
				Secret:                secretName,
			},
		}
	}

	HotBackup = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.HotBackup {
		return &hazelcastv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	CronHotBackup = func(lk types.NamespacedName, schedule string, hbSpec *hazelcastv1alpha1.HotBackupSpec, lbls map[string]string) *hazelcastv1alpha1.CronHotBackup {
		return &hazelcastv1alpha1.CronHotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.CronHotBackupSpec{
				Schedule: schedule,
				HotBackupTemplate: hazelcastv1alpha1.HotBackupTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Labels: lbls,
					},
					Spec: *hbSpec,
				},
			},
		}
	}

	Faulty = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              "not-exists",
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastv1alpha1.LoggingLevelDebug,
			},
		}
	}

	DefaultMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.Map {
		return &hazelcastv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	PersistedMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.Map {
		return &hazelcastv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
					BackupCount:           pointer.Int32(0),
				},
				PersistenceEnabled: true,
			},
		}
	}

	Map = func(ms hazelcastv1alpha1.MapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.Map {
		return &hazelcastv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: ms,
		}
	}

	BackupCountMap = func(lk types.NamespacedName, hzName string, lbls map[string]string, backupCount int32) *hazelcastv1alpha1.Map {
		return &hazelcastv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.MapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
					BackupCount:           &backupCount,
				},
			},
		}
	}

	DefaultWanReplication = func(wan types.NamespacedName, mapName, targetClusterName, endpoints string, lbls map[string]string) *hazelcastv1alpha1.WanReplication {
		return &hazelcastv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources: []hazelcastv1alpha1.ResourceSpec{{
					Name: mapName,
					Kind: hazelcastv1alpha1.ResourceKindMap,
				}},
			},
		}
	}

	CustomWanReplication = func(wan types.NamespacedName, targetClusterName, endpoints string, lbls map[string]string) *hazelcastv1alpha1.WanReplication {
		return &hazelcastv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources:         []hazelcastv1alpha1.ResourceSpec{},
			},
		}
	}

	WanReplication = func(wan types.NamespacedName, targetClusterName, endpoints string, resources []hazelcastv1alpha1.ResourceSpec, lbls map[string]string) *hazelcastv1alpha1.WanReplication {
		return &hazelcastv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources:         resources,
			},
		}
	}

	DefaultMultiMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.MultiMap {
		return &hazelcastv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.MultiMapSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	DefaultTopic = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.Topic {
		return &hazelcastv1alpha1.Topic{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.TopicSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	DefaultReplicatedMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.ReplicatedMap {
		return &hazelcastv1alpha1.ReplicatedMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.ReplicatedMapSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	DefaultQueue = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.Queue {
		return &hazelcastv1alpha1.Queue{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.QueueSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	DefaultCache = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.Cache {
		return &hazelcastv1alpha1.Cache{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.CacheSpec{
				DataStructureSpec: hazelcastv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				InMemoryFormat: hazelcastv1alpha1.InMemoryFormatBinary,
			},
		}
	}

	MultiMap = func(mms hazelcastv1alpha1.MultiMapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.MultiMap {
		return &hazelcastv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: mms,
		}
	}

	Topic = func(mms hazelcastv1alpha1.TopicSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.Topic {
		return &hazelcastv1alpha1.Topic{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: mms,
		}
	}

	ReplicatedMap = func(rms hazelcastv1alpha1.ReplicatedMapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.ReplicatedMap {
		return &hazelcastv1alpha1.ReplicatedMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: rms,
		}
	}

	Queue = func(qs hazelcastv1alpha1.QueueSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.Queue {
		return &hazelcastv1alpha1.Queue{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: qs,
		}
	}

	Cache = func(cs hazelcastv1alpha1.CacheSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.Cache {
		return &hazelcastv1alpha1.Cache{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: cs,
		}
	}

	JetJob = func(jarName string, hz string, lk types.NamespacedName, lbls map[string]string) *hazelcastv1alpha1.JetJob {
		return &hazelcastv1alpha1.JetJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.JetJobSpec{
				Name:                  lk.Name,
				HazelcastResourceName: hz,
				State:                 hazelcastv1alpha1.RunningJobState,
				JarName:               jarName,
			},
		}
	}
)

func repo(ee bool) string {
	if ee {
		return *hazelcastEERepo
	} else {
		return *hazelcastRepo
	}
}

func licenseKey(ee bool) string {
	if ee {
		return naming.LicenseKeySecret
	} else {
		return ""
	}
}

const (
	ExampleCert = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIUIAQAd7v+j7HF1ReAvOmcRnKvhyowDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHZXhhbXBsZTAeFw0yMzAzMzAyMTM0MTVaFw0zMzAzMjcy
MTM0MTVaMBIxEDAOBgNVBAMMB2V4YW1wbGUwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQClcwDpeKR8GPHSl43kNoARnZd0katAn1dLJA4xaumWmO6WTGMZ
CO5GkTA+4cfgLceSu/eRX0YKU8OdFjolQ3gB/qV/XEPkmrDet/fki8kwMxiDjxCA
TJ3BkOLwD1+kKjG/JYWGeSULa8osCQJoNttuY4Ep5cpZ1spLuJei0bItPZc4LRoq
pbblww1csIuz40xa6zM29nXwr/yVSk6gZLB0sqXlE1pLRwmm5yJVqwrb7yED63tp
5+R7E5Tths6NsvOuFlCfrLLLI8fjm7HECdnBlNVpfeSPP9gpmIGj8nqPSTelFVCe
VokegdMsiLXNmf/jAA5WLgF7nkvVPdw+bx1rAgMBAAGjUzBRMB0GA1UdDgQWBBQc
Jr/o86paLs6g7s02tfx27ALntTAfBgNVHSMEGDAWgBQcJr/o86paLs6g7s02tfx2
7ALntTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAEJZUFxde7
E2haX+zHTtFNe1reAtHiHL0oufJM4j77qhnbqBu1HzyDMekaNop18LFeI4wokINm
dhI6uWkog4r2oM0PGWys7fGSjxTdjf4imnsElbTDhVoCaUKpnPaP/TwcfHdZDdTB
dFddyuGVctC8+nGaHbQMT2IrRd7D0pOuSj5fMvgZmbUURSHFE1tzdjnH+uAAg+2+
FRxolQRHo8OvaWrProW8XMUEX5RD6fhtw/8OB3l66lKDy4fTSfyT3nulcmPsHUtS
R95Q5v7uTw/6roHb/By1jaQXtVkJ2WeM3wPO0IfWu02vFjjxpU4301Z19d7psF3t
+SnElJmXR3k5
-----END CERTIFICATE-----
	`

	ExampleKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQClcwDpeKR8GPHS
l43kNoARnZd0katAn1dLJA4xaumWmO6WTGMZCO5GkTA+4cfgLceSu/eRX0YKU8Od
FjolQ3gB/qV/XEPkmrDet/fki8kwMxiDjxCATJ3BkOLwD1+kKjG/JYWGeSULa8os
CQJoNttuY4Ep5cpZ1spLuJei0bItPZc4LRoqpbblww1csIuz40xa6zM29nXwr/yV
Sk6gZLB0sqXlE1pLRwmm5yJVqwrb7yED63tp5+R7E5Tths6NsvOuFlCfrLLLI8fj
m7HECdnBlNVpfeSPP9gpmIGj8nqPSTelFVCeVokegdMsiLXNmf/jAA5WLgF7nkvV
Pdw+bx1rAgMBAAECggEAKvvSYFW+Ehme9fH25LP+GNWDDD9uKQdctAJlh5Q5pK0N
y1GEK3RlB0NgL+4Tshviri4Udxm0BinV9+FW8Ohy7L2+PHT5lJJV4j8UcbWZauLT
exZ3mIWPNMNSGkE8PVfS/dCfPJ0LsUhrSX57uByMbMUAQSTYqfeCLiMCjkQBkPv0
QwZ0gGyDJh53OgpdA4HV2PNvQ7fAlb8MiT16wKDoh/dnm19L4jiAZhK1cK+IWX6L
3e5mCR4DNUF+JIXTfMgOKkf39/9Gb84svFbjw6Txoog8lxTBmm4ldElst1w5yQ2J
sTB8VOw7Yo+ime38qy95U/ySYAgISoXmUldm5B9vZQKBgQDmvTV4VslyAy5tVjIL
LGQYUdRbsrl2WP4CrB9ja2F2yhJ59gtK8FuKbhMuTqVWvoKyq0C97HRzZoJoPml5
ArcRlFv4jWPl81lVHvnS25gtBRA7TzDfovq35gZ2itOFJzfyqnmNWpGBt5h/HxWC
040pTZ8ZAmAUoLHYxgZ7oS4gJwKBgQC3j/QVWKUtp0d+8op0Uo8/inEiNcaeyYkX
O6Te850gBJ0JH2TuAaQ6OTWwyINYo6VhMNOjDr6i5WP6zc1iIigUnUU+o7uoDsIt
oK5J1OUY4uPk93dXUGT5A9wfZi2bwPtb9QUnKUTRSvIAVaQdgtrgG+jMLnU4F2aN
3SkABOFfHQKBgQDXjPQxiim/95bcj0RKydpsGa2fSDQXigUpK/BaqQqwtQ9TnfVo
uWdax3/lp5Svl2NzU6Y0hns2/xFeHsfbQx0QMB9G75beT1opubk6MOhVTkCel1kZ
4iADwcBR51i4MC4E5RqOYYhCvOeaAcjPoZ9icV/qNhzZyFC8KCoQPj9fywKBgBM4
+PePS+TnAp6xqXwa9TNTPRu3A/C27CtJrK9IVaj3srY02m3uMBOE0DGOHesXYAc4
hMErlx0Z5olqKdrf9tCJ06mGne0wdncuv3Gt4LvlbrYYkB/NpHVLSS7klVwdLnVn
yD1cnf9I2OTeEwygGmmjopJXPyE7mhq7EUMWP7+lAoGBAMJZ3sWM5vFKIbIWHITB
eI9mvE13AoKkYqWeX9bdxlYefScfKhuweFMhVbm9+x+317iTQcVjueC2swHvxkHC
fFN8odcHpU+Fn5G00adcVcwqKoWx3RJPKUrs3GHiKKZhnZNw0niNxONm54k3zDrO
psSqtGKFs43q0BlH1z1zjcN+
-----END PRIVATE KEY-----
	`
)
