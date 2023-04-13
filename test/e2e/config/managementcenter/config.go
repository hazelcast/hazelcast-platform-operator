package managementcenter

import (
	"flag"

	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var (
	mcVersion = flag.String("mc-version", naming.MCVersion, "Default Management Center version used in e2e tests")
	mcRepo    = flag.String("mc-repo", naming.MCRepo, "Management Center repository used in e2e tests")
)

var (
	Default = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1beta1.ManagementCenter {
		return &hazelcastv1beta1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1beta1.ManagementCenterSpec{
				Repository:       *mcRepo,
				Version:          *mcVersion,
				LicenseKeySecret: licenseKey(ee),
				ExternalConnectivity: hazelcastv1beta1.ExternalConnectivityConfiguration{
					Type: hazelcastv1beta1.ExternalConnectivityTypeLoadBalancer,
				},
				Persistence: hazelcastv1beta1.PersistenceConfiguration{
					Enabled: pointer.Bool(true),
					Size:    &[]resource.Quantity{resource.MustParse("10Gi")}[0],
				},
			},
		}
	}

	PersistenceDisabled = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1beta1.ManagementCenter {
		return &hazelcastv1beta1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1beta1.ManagementCenterSpec{
				Repository:       *mcRepo,
				Version:          *mcVersion,
				LicenseKeySecret: licenseKey(ee),
				ExternalConnectivity: hazelcastv1beta1.ExternalConnectivityConfiguration{
					Type: hazelcastv1beta1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: []hazelcastv1beta1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: hazelcastv1beta1.PersistenceConfiguration{
					Enabled: pointer.Bool(false),
				},
			},
		}
	}

	WithClusterConfig = func(lk types.NamespacedName, ee bool, clusterConfigs []hazelcastv1beta1.HazelcastClusterConfig, lbls map[string]string) *hazelcastv1beta1.ManagementCenter {
		return &hazelcastv1beta1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1beta1.ManagementCenterSpec{
				Repository:       *mcRepo,
				Version:          *mcVersion,
				LicenseKeySecret: licenseKey(ee),
				ExternalConnectivity: hazelcastv1beta1.ExternalConnectivityConfiguration{
					Type: hazelcastv1beta1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: clusterConfigs,
			},
		}
	}

	RouteEnabled = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1beta1.ManagementCenter {
		return &hazelcastv1beta1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1beta1.ManagementCenterSpec{
				Repository:       *mcRepo,
				Version:          *mcVersion,
				LicenseKeySecret: licenseKey(ee),
				ExternalConnectivity: hazelcastv1beta1.ExternalConnectivityConfiguration{

					Type: hazelcastv1beta1.ExternalConnectivityTypeClusterIP,
					Route: &hazelcastv1beta1.ExternalConnectivityRoute{
						Hostname: "",
					},
				},
				HazelcastClusters: []hazelcastv1beta1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: hazelcastv1beta1.PersistenceConfiguration{
					Enabled: pointer.Bool(false),
				},
			},
		}
	}

	Faulty = func(lk types.NamespacedName, ee bool, lbls map[string]string) *hazelcastv1beta1.ManagementCenter {
		return &hazelcastv1beta1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastv1beta1.ManagementCenterSpec{
				Repository:       *mcRepo,
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
