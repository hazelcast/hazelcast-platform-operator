package platform

import (
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type PlatformType string

const (
	OpenShift  PlatformType = "OpenShift"
	Kubernetes PlatformType = "Kubernetes"
)

type Platform struct {
	Type    PlatformType `json:"type"`
	Version string       `json:"version"`
}

var (
	plt Platform
)

func GetPlatform() (Platform, error) {
	if plt != (Platform{}) {
		return plt, nil
	}

	var err error
	plt, err = getInfo()
	if err != nil {
		return plt, err
	}
	return plt, nil
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

func getInfo() (Platform, error) {
	info := Platform{Type: Kubernetes}
	config, err := config.GetConfig()
	if err != nil {
		return info, err
	}

	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return info, err
	}

	k8sVersion, err := client.ServerVersion()
	if err != nil {
		return info, err
	}
	info.Version = k8sVersion.Major + "." + k8sVersion.Minor

	apiList, err := client.ServerGroups()
	if err != nil {
		return info, err
	}

	for _, v := range apiList.Groups {
		if v.Name == "route.openshift.io" {
			info.Type = OpenShift
			break
		}
	}
	return info, nil
}
