package managementcenter

import (
	"flag"

	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

var (
	mcVersion = flag.String("mc-version", naming.MCVersion, "Default Management Center version used in e2e tests")
	mcRepo    = flag.String("mc-repo", naming.MCRepo, "Management Center repository used in e2e tests")
)

var (
	Default = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					Enabled: pointer.Bool(true),
					Size:    &[]resource.Quantity{resource.MustParse("10Gi")}[0],
				},
			},
		}
	}

	PersistenceDisabled = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: []hazelcastcomv1alpha1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					Enabled: pointer.Bool(false),
				},
			},
		}
	}

	WithClusterConfig = func(lk types.NamespacedName, clusterConfigs []hazelcastcomv1alpha1.HazelcastClusterConfig, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{
					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeLoadBalancer,
				},
				HazelcastClusters: clusterConfigs,
			},
		}
	}

	RouteEnabled = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              *mcVersion,
				LicenseKeySecretName: naming.LicenseKeySecret,
				ExternalConnectivity: &hazelcastcomv1alpha1.ExternalConnectivityConfiguration{

					Type: hazelcastcomv1alpha1.ExternalConnectivityTypeClusterIP,
					Route: &hazelcastcomv1alpha1.ExternalConnectivityRoute{
						Hostname: "",
					},
				},
				HazelcastClusters: []hazelcastcomv1alpha1.HazelcastClusterConfig{
					{
						Name:    "dev",
						Address: "hazelcast",
					},
				},
				Persistence: &hazelcastcomv1alpha1.MCPersistenceConfiguration{
					Enabled: pointer.Bool(false),
				},
			},
		}
	}

	Faulty = func(lk types.NamespacedName, lbls map[string]string) *hazelcastcomv1alpha1.ManagementCenter {
		return &hazelcastcomv1alpha1.ManagementCenter{
			ObjectMeta: v1.ObjectMeta{
				Name:      lk.Name,
				Namespace: lk.Namespace,
				Labels:    lbls,
			},
			Spec: hazelcastcomv1alpha1.ManagementCenterSpec{
				Repository:           *mcRepo,
				Version:              "not-exists",
				LicenseKeySecretName: naming.LicenseKeySecret,
			},
		}
	}
)
