package hazelcast

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

var (
	ClusterName = func(ns string, ee bool) *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
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
				ClusterSize:      3,
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
				ClusterSize:      3,
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
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
				ClusterSize:      3,
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
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
				ClusterSize:      3,
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
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
				ClusterSize:      3,
				Repository:       repo(ee),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}

	PersistenceEnabled = func(ns string, baseDir string, useHostPath bool) *hazelcastv1alpha1.Hazelcast {
		hz := &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      "hazelcast",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
				Repository:       repo(true),
				Version:          naming.HazelcastVersion,
				LicenseKeySecret: licenseKey(true),
				Persistence: hazelcastv1alpha1.HazelcastPersistenceConfiguration{
					BaseDir:                   baseDir,
					ClusterDataRecoveryPolicy: hazelcastv1alpha1.FullRecovery,
					Pvc: hazelcastv1alpha1.PersistencePvcConfiguration{
						AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						RequestStorage: resource.MustParse("8Gi"),
					},
				},
			},
		}

		if useHostPath {
			hz.Spec.Persistence.HostPath = "/tmp/hazelcast"
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
				ClusterSize:      3,
				Repository:       repo(ee),
				Version:          "not-exists",
				LicenseKeySecret: licenseKey(ee),
			},
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
