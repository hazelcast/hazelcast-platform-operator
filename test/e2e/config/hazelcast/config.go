package hazelcast

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
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
				ClusterSize:      &[]int32{3}[0],
				ClusterName:      "development",
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
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
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
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
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
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
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
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
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
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
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: &hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	PersistenceEnabled = func(lk types.NamespacedName, baseDir string, lbls map[string]string, params ...interface{}) *hazelcastv1alpha1.Hazelcast {
		var hostPath, nodeName string
		var hok, nok bool
		if len(params) > 0 {
			hostPath, hok = params[0].(string)
		}
		if len(params) > 1 {
			nodeName, nok = params[1].(string)
		}
		hz := &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(true),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(true),
				Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   baseDir,
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					},
				},
			},
		}

		if hok {
			hz.Spec.Persistence.HostPath = hostPath
			hz.Spec.Persistence.Pvc = hazelcastv1alpha1.PersistencePvcConfiguration{}
			hz.Spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{
						MaxSkew:           int32(1),
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: corev1.DoNotSchedule,
						LabelSelector: &v1.LabelSelector{
							MatchLabels: map[string]string{
								naming.ApplicationNameLabel:         naming.Hazelcast,
								naming.ApplicationInstanceNameLabel: hz.Name,
								naming.ApplicationManagedByLabel:    naming.OperatorName,
							},
						},
					},
				},
			}
		}

		if nok {
			hz.Spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
				NodeSelector: map[string]string{
					"kubernetes.io/hostname": nodeName,
				},
			}
		}
		return hz
	}

	ExternalBackup = func(lk types.NamespacedName, ee bool, labels map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      &[]int32{1}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   "/data/hot-restart",
					BackupType:                "External",
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					},
				},
			},
		}
	}

	ExternalRestore = func(lk types.NamespacedName, ee bool, labels map[string]string, bucketURI, secretName string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      &[]int32{1}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				Persistence: &hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   "/data/hot-restart",
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: &[]resource.Quantity{resource.MustParse("8Gi")}[0],
					},
					Restore: &hazelcastv1alpha1.RestoreConfiguration{
						BucketURI: bucketURI,
						Secret:    secretName,
					},
				},
			},
		}
	}

	UserCode = func(lk types.NamespacedName, ee bool, s, bkt string, lbls map[string]string) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      &[]int32{1}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				UserCodeDeployment: &hazelcastv1alpha1.UserCodeDeploymentConfig{
					BucketConfiguration: &hazelcastv1alpha1.BucketConfiguration{
						Secret:    s,
						BucketURI: bkt,
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
				ClusterSize:               &[]int32{1}[0],
				Repository:                repo(ee),
				Version:                   naming.HazelcastVersion,
				LicenseKeySecret:          licenseKey(ee),
				ExecutorServices:          allExecutorServices["es"].([]hazelcastv1alpha1.ExecutorServiceConfiguration),
				DurableExecutorServices:   allExecutorServices["des"].([]hazelcastv1alpha1.DurableExecutorServiceConfiguration),
				ScheduledExecutorServices: allExecutorServices["ses"].([]hazelcastv1alpha1.ScheduledExecutorServiceConfiguration),
			},
		}
	}

	HotBackupAgent = func(lk types.NamespacedName, hzName string, lbls map[string]string, bucketURI, secretName string) *hazelcastv1alpha1.HotBackup {
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
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          "not-exists",
				LicenseKeySecret: licenseKey(ee),
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
				HazelcastResourceName: hzName,
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

	DefaultMultiMap = func(lk types.NamespacedName, hzName string, lbls map[string]string) *hazelcastv1alpha1.MultiMap {
		return &hazelcastv1alpha1.MultiMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1alpha1.MultiMapSpec{
				HazelcastResourceName: hzName,
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
)

func repo(ee bool) string {
	if ee {
		return naming.HazelcastEERepo
	} else {
		return naming.HazelcastRepo
	}
}

func licenseKey(ee bool) string {
	if ee {
		return naming.LicenseKeySecret
	} else {
		return ""
	}
}
