package hazelcast

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var (
	ClusterName = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
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

	Default = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
			},
		}
	}

	ExposeExternallySmartLoadBalancer = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
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

	ExposeExternallySmartNodePort = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
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

	ExposeExternallySmartNodePortNodeName = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
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

	ExposeExternallyUnisocket = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
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

	PersistenceEnabled = func(ns, baseDir string, params ...interface{}) *hazelcastv1alpha1.Hazelcast {
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
				Name:      "hazelcast",
				Namespace: ns,
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
			hz.Spec.Scheduling = &hazelcastv1alpha1.SchedulingConfiguration{
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
					{
						MaxSkew:           int32(1),
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: corev1.DoNotSchedule,
						LabelSelector: &v1.LabelSelector{
							MatchLabels: map[string]string{
								naming.ApplicationNameLabel:         naming.Hazelcast,
								naming.ApplicationInstanceNameLabel: "hazelcast",
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

	HotBackup = func(hzName, ns string) *hazelcastv1alpha1.HotBackup {
		return &hazelcastv1alpha1.HotBackup{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hot-backup",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.HotBackupSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	Faulty = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      &[]int32{3}[0],
				Repository:       repo(ee),
				Version:          "not-exists",
				LicenseKeySecret: licenseKey(ee),
			},
		}
	}

	DefaultMap = func(hzName, mapName, ns string) *hazelcastv1alpha1.Map {
		return &hazelcastv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      mapName,
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.MapSpec{
				HazelcastResourceName: hzName,
			},
		}
	}

	Map = func(ms hazelcastv1alpha1.MapSpec, mapName, ns string) *hazelcastv1alpha1.Map {
		return &hazelcastv1alpha1.Map{
			ObjectMeta: v1.ObjectMeta{
				Name:      mapName,
				Namespace: ns,
			},
			Spec: ms,
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
