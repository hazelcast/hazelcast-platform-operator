package hazelcast

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var (
	ClusterName = func() *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name: "hazelcast",
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
				ClusterName:      "development",
				Repository:       repo(),
				Version:          "5.0",
				LicenseKeySecret: licenseKey(),
			},
		}
	}

	Default = func() *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name: "hazelcast",
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
				Repository:       repo(),
				Version:          "5.0",
				LicenseKeySecret: licenseKey(),
			},
		}
	}

	ExposeExternallySmartLoadBalancer = func() *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name: "hazelcast",
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
				Repository:       repo(),
				Version:          "latest-snapshot-slim",
				LicenseKeySecret: licenseKey(),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessLoadBalancer,
				},
			},
		}
	}

	ExposeExternallySmartNodePort = func() *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name: "hazelcast",
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
				Repository:       repo(),
				Version:          "latest-snapshot-slim",
				LicenseKeySecret: licenseKey(),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeSmart,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
					MemberAccess:         hazelcastv1alpha1.MemberAccessNodePortExternalIP,
				},
			},
		}
	}

	ExposeExternallyUnisocket = func() *hazelcastv1alpha1.Hazelcast {
		return &hazelcastv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name: "hazelcast",
			},
			Spec: hazelcastv1alpha1.HazelcastSpec{
				ClusterSize:      3,
				Repository:       repo(),
				Version:          "latest-snapshot-slim",
				LicenseKeySecret: licenseKey(),
				ExposeExternally: hazelcastv1alpha1.ExposeExternallyConfiguration{
					Type:                 hazelcastv1alpha1.ExposeExternallyTypeUnisocket,
					DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				},
			},
		}
	}
)

func repo() string {
	if test.IsEE() {
		return naming.HazelcastEERepo
	} else {
		return naming.HazelcastRepo
	}
}

func licenseKey() string {
	if test.IsEE() {
		return naming.LicenseKeySecret
	} else {
		return ""
	}
}
