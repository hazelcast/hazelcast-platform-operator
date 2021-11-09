package hazelcast

import (
	corev1 "k8s.io/api/core/v1"
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
				Version:          "5.0",
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
				Version:          "5.0",
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
				Version:          "latest-snapshot-slim",
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
				Version:          "latest-snapshot-slim",
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
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
				Version:          "latest-snapshot-slim",
				LicenseKeySecret: licenseKey(ee),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
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
