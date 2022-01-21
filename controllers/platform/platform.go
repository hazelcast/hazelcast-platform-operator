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
	cfg *rest.Config
)

func SetClientConfig(c *rest.Config) {
	cfg = c
}

func GetPlatform() (Platform, error) {
	if plt != (Platform{}) {
		return plt, nil
	}

	var err error
	plt, err = getInfo()
	if err != nil {
		return Platform{}, err
	}
	return plt, nil
}

func GetProvider() (ProviderType, error) {
	if plt != (Platform{}) {
		return plt.Provider, nil
	}

	var err error
	plt, err = getInfo()
	if err != nil {
		return "", err
	}
	return plt.Provider, nil
}

func GetType() (PlatformType, error) {
	if plt.Type != "" {
		return plt.Type, nil
	}

	var err error
	plt, err = getInfo()
	if err != nil {
		return "", err
	}
	return plt.Type, nil
}

func GetVersion() (string, error) {
	if plt.Version != "" {
		return plt.Version, nil
	}

	var err error
	plt, err = getInfo()
	if err != nil {
		return "", err
	}
	return plt.Version, nil
}

func newClient() (*discovery.DiscoveryClient, error) {

	if cfg != nil {
		return discovery.NewDiscoveryClientForConfig(cfg)
	}

	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	return discovery.NewDiscoveryClientForConfig(config)

}

func getInfo() (Platform, error) {
	info := Platform{Type: Kubernetes}

	client, err := newClient()
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
