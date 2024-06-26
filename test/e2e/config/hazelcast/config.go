package hazelcast

import (
	"flag"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var (
	hazelcastVersion = flag.String("hazelcast-version", naming.HazelcastVersion, "Default Hazelcast version used in e2e tests")
	hazelcastEERepo  = flag.String("hazelcast-ee-repo", naming.HazelcastEERepo, "Enterprise Hazelcast repository used in e2e tests")
	hazelcastRepo    = flag.String("hazelcast-os-repo", naming.HazelcastRepo, "Hazelcast repository used in e2e tests")
)

var (
	ClusterName = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				ClusterName:          "development",
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			},
		}
	}

	Default = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			},
		}
	}

	ExposeExternallySmartLoadBalancer = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
				},
			},
		}
	}

	ExposeExternallySmartNodePort = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastcomv1alpha1.MemberAccessNodePortExternalIP,
				},
			},
		}
	}

	ExposeExternallySmartNodePortNodeName = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeNodePort,
					MemberAccess:         hazelcastcomv1alpha1.MemberAccessNodePortNodeName,
				},
			},
		}
	}

	ExposeExternallyUnisocket = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				LicenseKeySecretName: licenseKey(ee),
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	HazelcastPersistencePVC = func(lk types.NamespacedName, clusterSize int32, labels map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(clusterSize),
				Repository:           repo(true),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(true),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					},
				},
			},
		}
	}

	CPSubsystem = func(clusterSize int32) hazelcastcomv1alpha1.HazelcastSpec {
		return hazelcastcomv1alpha1.HazelcastSpec{
			ClusterSize:          pointer.Int32(clusterSize),
			Repository:           repo(true),
			Version:              "5.5.0-SNAPSHOT",
			LicenseKeySecretName: licenseKey(true),
			LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			CPSubsystem: &hazelcastcomv1alpha1.CPSubsystem{
				PVC: &hazelcastcomv1alpha1.PvcConfiguration{
					AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
				},
			},
		}
	}

	CPSubsystemPersistence = func(clusterSize int32) hazelcastcomv1alpha1.HazelcastSpec {
		return hazelcastcomv1alpha1.HazelcastSpec{
			ClusterSize:          pointer.Int32(clusterSize),
			Repository:           repo(true),
			Version:              "5.5.0-SNAPSHOT",
			LicenseKeySecretName: licenseKey(true),
			LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
				PVC: &hazelcastcomv1alpha1.PvcConfiguration{
					AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
				},
			},
			CPSubsystem: &hazelcastcomv1alpha1.CPSubsystem{},
		}
	}

	HazelcastRestore = func(hz *hazelcastcomv1alpha1.Hazelcast, restoreConfig hazelcastcomv1alpha1.RestoreConfiguration) *hazelcastcomv1alpha1.Hazelcast {
		hzRestore := &hazelcastcomv1alpha1.Hazelcast{
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

	UserCodeBucket = func(lk types.NamespacedName, ee bool, s, bkt string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{1}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				DeprecatedUserCodeDeployment: &hazelcastcomv1alpha1.UserCodeDeploymentConfig{
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
				},
			},
		}
	}

	JetConfigured = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
				},
			},
		}
	}

	JetWithBucketConfigured = func(lk types.NamespacedName, ee bool, s, bkt string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
				},
			},
		}
	}

	JetWithUrlConfigured = func(lk types.NamespacedName, ee bool, url string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						RemoteURLs: []string{url},
					},
				},
			},
		}
	}

	JetWithLosslessRestart = func(lk types.NamespacedName, ee bool, s, bkt string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
							SecretName: s,
							BucketURI:  bkt,
						},
					},
					Instance: &hazelcastcomv1alpha1.JetInstance{
						LosslessRestartEnabled:         true,
						CooperativeThreadCount:         pointer.Int32(1),
						MaxProcessorAccumulatedRecords: pointer.Int64(1000000000),
					},
				},
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: resource.NewQuantity(9*2^20, resource.BinarySI),
					},
				},
			},
		}
	}

	JetWithRestore = func(lk types.NamespacedName, ee bool, hbn string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(1),
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				JetEngineConfiguration: &hazelcastcomv1alpha1.JetEngineConfiguration{
					Enabled:               pointer.Bool(true),
					ResourceUploadEnabled: true,
					Instance: &hazelcastcomv1alpha1.JetInstance{
						LosslessRestartEnabled:         true,
						CooperativeThreadCount:         pointer.Int32(1),
						MaxProcessorAccumulatedRecords: pointer.Int64(1000000000),
					},
				},
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: resource.NewQuantity(9*2^20, resource.BinarySI),
					},
					Restore: hazelcastcomv1alpha1.RestoreConfiguration{
						HotBackupResourceName: hbn,
					},
				},
			},
		}
	}

	UserCodeURL = func(lk types.NamespacedName, ee bool, urls []string, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{1}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				DeprecatedUserCodeDeployment: &hazelcastcomv1alpha1.UserCodeDeploymentConfig{
					RemoteFileConfiguration: hazelcastcomv1alpha1.RemoteFileConfiguration{
						RemoteURLs: urls,
					},
				},
			},
		}
	}

	ExecutorService = func(lk types.NamespacedName, ee bool, allExecutorServices map[string]interface{}, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				LoggingLevel:              hazelcastcomv1alpha1.LoggingLevelDebug,
				ClusterSize:               &[]int32{1}[0],
				Repository:                repo(ee),
				Version:                   *hazelcastVersion,
				LicenseKeySecretName:      licenseKey(ee),
				ExecutorServices:          allExecutorServices["es"].([]hazelcastcomv1alpha1.ExecutorServiceConfiguration),
				DurableExecutorServices:   allExecutorServices["des"].([]hazelcastcomv1alpha1.DurableExecutorServiceConfiguration),
				ScheduledExecutorServices: allExecutorServices["ses"].([]hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration),
			},
		}
	}

	HighAvailability = func(lk types.NamespacedName, ee bool, size int32, mode hazelcastcomv1alpha1.HighAvailabilityMode, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &size,
				HighAvailabilityMode: mode,
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				ExposeExternally: &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	HazelcastTLS = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				TLS: &hazelcastcomv1alpha1.TLS{
					SecretName: lk.Name + "-tls",
				},
			},
		}
	}

	HazelcastMTLS = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(ee),
				TLS: &hazelcastcomv1alpha1.TLS{
					SecretName:           lk.Name + "-mtls",
					MutualAuthentication: hazelcastcomv1alpha1.MutualAuthenticationRequired,
				},
			},
		}
	}

	HazelcastSQLPersistence = func(lk types.NamespacedName, clusterSize int32, labels map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(clusterSize),
				Repository:           repo(true),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(true),
				Persistence: &hazelcastcomv1alpha1.HazelcastPersistenceConfiguration{
					ClusterDataRecoveryPolicy: hazelcastcomv1alpha1.FullRecovery,
					PVC: &hazelcastcomv1alpha1.PvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					},
				},
				SQL: &hazelcastcomv1alpha1.SQL{
					CatalogPersistenceEnabled: true,
				},
			},
		}
	}

	HotBackupBucket = func(lk types.NamespacedName, hzName string, lbls map[string]string, bucketURI, secretName string) *hazelcastcomv1alpha1.HotBackup {
		return &hazelcastcomv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
				BucketURI:             bucketURI,
				SecretName:            secretName,
			},
		}
	}

	HotBackup = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.HotBackup {
		return &hazelcastcomv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	CronHotBackup = func(lk types.NamespacedName, schedule string, hbSpec *hazelcastcomv1alpha1.HotBackupSpec, lbls map[string]string) *hazelcastcomv1alpha1.CronHotBackup {
		return &hazelcastcomv1alpha1.CronHotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.CronHotBackupSpec{
				Schedule: schedule,
				HotBackupTemplate: hazelcastcomv1alpha1.HotBackupTemplateSpec{
					ObjectMeta: v1.ObjectMeta{
						Labels: lbls,
					},
					Spec: *hbSpec,
				},
			},
		}
	}

	Faulty = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          &[]int32{3}[0],
				Repository:           repo(ee),
				Version:              "not-exists",
				LicenseKeySecretName: licenseKey(ee),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
			},
		}
	}

	DefaultMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	PersistedMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
					BackupCount:           pointer.Int32(0),
				},
				PersistenceEnabled: true,
			},
		}
	}

	Map = func(ms hazelcastcomv1alpha1.MapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: ms,
		}
	}

	BackupCountMap = func(lk types.NamespacedName, hzName string, lbls map[string]string, backupCount int32) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
					BackupCount:           &backupCount,
				},
			},
		}
	}

	MapWithEventJournal = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				EventJournal: &hazelcastcomv1alpha1.EventJournal{},
			},
		}
	}

	DefaultTieredStoreMap = func(lk types.NamespacedName, hzName string, deviceName string, lbls map[string]string) *hazelcastcomv1alpha1.Map {
		return &hazelcastcomv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				InMemoryFormat: hazelcastcomv1alpha1.InMemoryFormatNative,
				TieredStore: &hazelcastcomv1alpha1.TieredStore{
					DiskDeviceName: deviceName,
				},
			},
		}
	}

	DefaultWanReplication = func(wan types.NamespacedName, mapName, targetClusterName, endpoints string, lbls map[string]string) *hazelcastcomv1alpha1.WanReplication {
		return &hazelcastcomv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources: []hazelcastcomv1alpha1.ResourceSpec{{
					Name: mapName,
					Kind: hazelcastcomv1alpha1.ResourceKindMap,
				}},
			},
		}
	}

	CustomWanReplication = func(wan types.NamespacedName, targetClusterName, endpoints string, lbls map[string]string) *hazelcastcomv1alpha1.WanReplication {
		return &hazelcastcomv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources:         []hazelcastcomv1alpha1.ResourceSpec{},
			},
		}
	}

	WanReplication = func(wan types.NamespacedName, targetClusterName, endpoints string, resources []hazelcastcomv1alpha1.ResourceSpec, lbls map[string]string) *hazelcastcomv1alpha1.WanReplication {
		return &hazelcastcomv1alpha1.WanReplication{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanReplicationSpec{
				TargetClusterName: targetClusterName,
				Endpoints:         endpoints,
				Resources:         resources,
			},
		}
	}

	WanSync = func(wan types.NamespacedName, wanReplicationName string, lbls map[string]string) *hazelcastcomv1alpha1.WanSync {
		return &hazelcastcomv1alpha1.WanSync{
			ObjectMeta: v1.ObjectMeta{
				Name:      wan.Name,
				Namespace: wan.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.WanSyncSpec{
				WanReplicationResourceName: wanReplicationName,
			},
		}
	}

	DefaultMultiMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.MultiMap {
		return &hazelcastcomv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.MultiMapSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	DefaultTopic = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Topic {
		return &hazelcastcomv1alpha1.Topic{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.TopicSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	DefaultReplicatedMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.ReplicatedMap {
		return &hazelcastcomv1alpha1.ReplicatedMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ReplicatedMapSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	DefaultQueue = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Queue {
		return &hazelcastcomv1alpha1.Queue{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.QueueSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
			},
		}
	}

	DefaultCache = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastcomv1alpha1.Cache {
		return &hazelcastcomv1alpha1.Cache{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.CacheSpec{
				DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
					HazelcastResourceName: hzName,
				},
				InMemoryFormat: hazelcastcomv1alpha1.InMemoryFormatBinary,
			},
		}
	}

	MultiMap = func(mms hazelcastcomv1alpha1.MultiMapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.MultiMap {
		return &hazelcastcomv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: mms,
		}
	}

	Topic = func(mms hazelcastcomv1alpha1.TopicSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Topic {
		return &hazelcastcomv1alpha1.Topic{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: mms,
		}
	}

	ReplicatedMap = func(rms hazelcastcomv1alpha1.ReplicatedMapSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ReplicatedMap {
		return &hazelcastcomv1alpha1.ReplicatedMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: rms,
		}
	}

	Queue = func(qs hazelcastcomv1alpha1.QueueSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Queue {
		return &hazelcastcomv1alpha1.Queue{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: qs,
		}
	}

	Cache = func(cs hazelcastcomv1alpha1.CacheSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.Cache {
		return &hazelcastcomv1alpha1.Cache{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: cs,
		}
	}

	JetJob = func(jarName string, hz string, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.JetJob {
		return &hazelcastcomv1alpha1.JetJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.JetJobSpec{
				Name:                  lk.Name,
				HazelcastResourceName: hz,
				State:                 hazelcastcomv1alpha1.RunningJobState,
				JarName:               jarName,
			},
		}
	}

	JetJobWithInitialSnapshot = func(jarName string, hz string, snapshotResourceName string, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.JetJob {
		return &hazelcastcomv1alpha1.JetJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.JetJobSpec{
				Name:                        lk.Name,
				HazelcastResourceName:       hz,
				State:                       hazelcastcomv1alpha1.RunningJobState,
				JarName:                     jarName,
				InitialSnapshotResourceName: snapshotResourceName,
			},
		}
	}

	JetJobSnapshot = func(name string, cancel bool, jetJobResourceName string, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.JetJobSnapshot {
		return &hazelcastcomv1alpha1.JetJobSnapshot{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.JetJobSnapshotSpec{
				Name:               name,
				CancelJob:          cancel,
				JetJobResourceName: jetJobResourceName,
			},
		}
	}

	TLSSecret = func(lk types.NamespacedName, lbls map[string]string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				corev1.TLSCertKey:       []byte(ExampleCert),
				corev1.TLSPrivateKeyKey: []byte(ExampleKey),
			},
		}
	}

	HazelcastTieredStorage = func(lk types.NamespacedName, deviceName string, labels map[string]string) *hazelcastcomv1alpha1.Hazelcast {
		return &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastcomv1alpha1.HazelcastSpec{
				ClusterSize:          pointer.Int32(3),
				Repository:           repo(true),
				Version:              *hazelcastVersion,
				LicenseKeySecretName: licenseKey(true),
				LoggingLevel:         hazelcastcomv1alpha1.LoggingLevelDebug,
				NativeMemory: &hazelcastcomv1alpha1.NativeMemoryConfiguration{
					AllocatorType: hazelcastcomv1alpha1.NativeMemoryStandard,
				},
				LocalDevices: []hazelcastcomv1alpha1.LocalDeviceConfig{
					{
						Name: deviceName,
						PVC: &hazelcastcomv1alpha1.PvcConfiguration{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						},
					},
				},
			},
		}
	}

	UserCodeNamespace = func(ucns hazelcastcomv1alpha1.UserCodeNamespaceSpec, lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.UserCodeNamespace {
		return &hazelcastcomv1alpha1.UserCodeNamespace{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: ucns,
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
MIIDJDCCAgygAwIBAgIUKzKxkelznHkzTRJcAff41uYNOzwwDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHZXhhbXBsZTAeFw0yMzA1MDQxMjI1NTVaFw0zMzA1MDEx
MjI1NTVaMBIxEDAOBgNVBAMMB2V4YW1wbGUwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCtdKO42gtHph8X+Q5jIBVAuOfR9nGWZoaLuF5+741CTihygmqr
WBAxkxVmpIgD+kHsX04hC4ku4uyBEncRjWAtncH+3/AYYZ3/QC+l1/PMX5WfiJ8X
lEYxuOb0d86ZjgVdWVgRi3qyePHmdzBnPTFOfF5lc5SDdIWIbJ37/y0Ar5Wivftx
QMqzfLdK9cAdW3yd/D3tzfVlIHk1NarVJVxnwpfvvtAoGj+JkQ/ZGu9qrpgpLPOH
vsY0AuL1gEaNgHYTCZfkWsDklobceBaHB3boAKf8k91Xou9J6rAMHT8+clFgPPuV
jk5ws4eOgIgBFOL4zztqVaR9ZSsSvGfLWrSTAgMBAAGjcjBwMB0GA1UdDgQWBBQX
eKHFreouZ5JhhocAynjN94i2tjAfBgNVHSMEGDAWgBQXeKHFreouZ5JhhocAynjN
94i2tjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEF
BQcDAjANBgkqhkiG9w0BAQsFAAOCAQEAXFHpdAZmxMAWZX2P65c2kNdSUu8dfKmp
GO0HbInAY/nnaKVPwwKs3J58DMQON7a4RXyMS8s3l6DlVwdxbGrBdc74fCFgGtRX
R4e5B6O4kGYedFx1GlFlbShzWSCu3RUjMPZ7bQlqELtXGh9Zz7sE0MZqJsTLgnDd
E8m2YHdnzHmvbwprs9z4J8vsbUZL/zWheWCwergogEKA9sqUf82jUHKlLPELDykb
/tZkWIEmH3HZ3iIzv1W0aq+TcjfL5Pm+OBG36KgyLg0jJJ6G3rj+NqWyeGZYcN0P
j7p1jMEkX90CJIweXgzJvPJ1UcpP7amZCHk4N2adz8QVRee4DRECmw==
-----END CERTIFICATE-----
	`

	ExampleKey = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCtdKO42gtHph8X
+Q5jIBVAuOfR9nGWZoaLuF5+741CTihygmqrWBAxkxVmpIgD+kHsX04hC4ku4uyB
EncRjWAtncH+3/AYYZ3/QC+l1/PMX5WfiJ8XlEYxuOb0d86ZjgVdWVgRi3qyePHm
dzBnPTFOfF5lc5SDdIWIbJ37/y0Ar5WivftxQMqzfLdK9cAdW3yd/D3tzfVlIHk1
NarVJVxnwpfvvtAoGj+JkQ/ZGu9qrpgpLPOHvsY0AuL1gEaNgHYTCZfkWsDklobc
eBaHB3boAKf8k91Xou9J6rAMHT8+clFgPPuVjk5ws4eOgIgBFOL4zztqVaR9ZSsS
vGfLWrSTAgMBAAECggEAOHAXRXJM8UcwHtC+yaoKwEBpzXtugg1iAdw/gvXW9JgR
uRCOPKouurKs5/To/MJU6OApv77NKCBV67liXKevf6gxEwkySfyZOBBecIvPm9QO
DxaZDUcFf/A11Z2V74iyXilP6oWDqsaHjwGBElZq0KrO3Bu7Wvpy6GzPCsuAjRQK
iELvY0E57RZvie5aHtZKdhV8ZbtC40dZzO9hboBZUv1kdEyf8bmcKYseB8WRjmxj
Z2BsqxS1reMzaVJX/qC3294Cpyv6G4zioJ0AKUijUZk+HhK5dB14asA1xfC394DR
gUDthuEZtRIAA/DuMhuyG+n2xoEWoagN7uU0Lei3YQKBgQC76sH0ygWqAKZxPWLV
Co7NxC0GN4U+FbWQDQ0ohGHqNV8BBS7Ja5OaQyr9HF8AEYiAWDa2jZYDBQkGYJNK
F3IvOdmez/DQT3/qPUbqxfVoJQGAnLmGrEPYzCM9Wns8OQsVZmcG0TfZrd7u5tDm
rcOBF7m5BXF5NPIBCxD7+fvBTwKBgQDsTJeFsMeffomsMHZkpfqfn6gZRq/0K512
xHPTWjPE8emG22AAnAuLi7/s9UJ5UAyrAOOOtwJicCFFdA7r1Y+dkTq/WCPQUWQL
JfGIpbyrSIlQS+n/w+xProNMx4xtbtkUek+PWVC8C9NuRvXb+HzKbPflaGYBFm2A
rCbk7/9ffQKBgDIGykHHsoBSkfzdkb0ThXbj/fSEvVUM5HwH7XPW4lY+hR85aP44
RGAx93TQo73Z7RP16ALraH8/TOrEtRFpcn1+EiBETWC3eV87lvCTaMSj7WV207E1
lQ5XMh54QwyCRyAYVd8rvYmWzx2clwqCQeTREyFdgJr67F44uvnJ0CrjAoGAYyZk
MdGSgYcL53dSRjsq5U2NsEVr0S133fziiN2BeXL0RQTJzJetdHlIJ/plURfYqOwv
j5OU6Y8ZNtZS6HvszfXBS8aFCIUOUGs0ZNz+RHSkQVAJOKuR/YFBULcuYkCvz5re
xUx5xt3DcrNNuGYUnq+IePcMTgqGGgaiL0/QvNUCgYB+BU9WvRfLtY8SPa8HEyQm
XNNtM+ABirkTi+Vym4Z1Y48ky1kBM5PuCFAbndlPvWSzcwy5jd+obB4mD4Q/wSVz
BJV1B7N4JQrVXBpOKW9TtJdHQ2WoTsx8AAnFEZ+C2BfNg+jqgvPtgZpwIcY+wcqG
nq1euTfl0SHp6nrtumTvwg==
-----END PRIVATE KEY-----
`
)
