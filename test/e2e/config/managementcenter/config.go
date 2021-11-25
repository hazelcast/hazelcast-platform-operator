package managementcenter

import (
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

var (
	Default = func(ns string, ee bool) *hazelcastv1alpha1.ManagementCenter {
		return &hazelcastv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      "managementcenter",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.ManagementCenterSpec{
				Repository:       naming.MCRepo,
				Version:          naming.MCVersion,
				LicenseKeySecret: licenseKey(ee),
				ExternalConnectivity: hazelcastv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: hazelcastv1alpha1.PersistenceConfiguration{
					Enabled: true,
					Size:    resource.MustParse("10Gi"),
				},
			},
		}
	}

	PersistenceDisabled = func(ns string, ee bool) *hazelcastv1alpha1.ManagementCenter {
		return &hazelcastv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      "managementcenter",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.ManagementCenterSpec{
				Repository:       naming.MCRepo,
				Version:          naming.MCVersion,
				LicenseKeySecret: licenseKey(ee),
				ExternalConnectivity: hazelcastv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: hazelcastv1alpha1.PersistenceConfiguration{
					Enabled: false,
				},
			},
		}
	}

	Faulty = func(ns string, ee bool) *hazelcastv1alpha1.ManagementCenter {
		return &hazelcastv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      "managementcenter",
				Namespace: ns,
			},
			Spec: hazelcastv1alpha1.ManagementCenterSpec{
				Repository:       naming.MCRepo,
				Version:          "not-exists",
				LicenseKeySecret: licenseKey(ee),
			},
		}
	}
)

func licenseKey(ee bool) string {
	if ee {
		return naming.LicenseKeySecret
	} else {
		return ""
	}
}
