package platform

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Platform struct {
	Type     PlatformType `json:"type"`
	Provider ProviderType `json:"provider"`
	Version  string       `json:"version"`
}

type PlatformType string

const (
	OpenShift  PlatformType = "OpenShift"
	Kubernetes PlatformType = "Kubernetes"
)

type ProviderType string

const (
	EKS ProviderType = "EKS"
	AKS ProviderType = "AKS"
	GKE ProviderType = "GKE"
)

var (
	plt Platform
)

func GetPlatform() Platform {
	return plt
}

func GetType() PlatformType {
	return plt.Type
}

func GetProvider() ProviderType {
	return plt.Provider
}

func GetVersion() string {
	return plt.Version
}

func FindAndSetPlatform(cfg *rest.Config) error {
	info, err := GetPlatformInfo(cfg)
	if err != nil {
		return err
	}
	plt = info
	return nil
}

func GetPlatformInfo(cfg *rest.Config) (Platform, error) {
	info := Platform{Type: Kubernetes}

	client, err := newClient(cfg)
	if err != nil {
		return Platform{}, err
	}

	k8sVersion, err := client.ServerVersion()
	if err != nil {
		return Platform{}, err
	}

	info.Version = k8sVersion.Major + "." + k8sVersion.Minor

	apiList, err := client.ServerGroups()
	if err != nil {
		return Platform{}, err
	}

	for _, v := range apiList.Groups {
		if v.Name == "route.openshift.io" {
			info.Type = OpenShift
			return info, nil
		}
	}

	for _, v := range apiList.Groups {
		switch v.Name {
		case "crd.k8s.amazonaws.com":
			info.Provider = EKS
			return info, nil
		case "networking.gke.io":
			info.Provider = GKE
			return info, nil
		case "azmon.container.insights":
			info.Provider = AKS
			return info, nil
		}
	}

	return info, nil
}

func newClient(cfg *rest.Config) (*discovery.DiscoveryClient, error) {
	if cfg != nil {
		return discovery.NewDiscoveryClientForConfig(cfg)
	}

	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	return discovery.NewDiscoveryClientForConfig(config)
}
