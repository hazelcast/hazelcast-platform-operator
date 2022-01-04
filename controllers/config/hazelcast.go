package config

type HazelcastWrapper struct {
	Hazelcast Hazelcast `yaml:"hazelcast"`
}

type Hazelcast struct {
	Jet         Jet     `yaml:"jet,omitempty"`
	Network     Network `yaml:"network,omitempty"`
	ClusterName string  `yaml:"cluster-name,omitempty"`
}

type Jet struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

type Network struct {
	Join    Join    `yaml:"join,omitempty"`
	RestAPI RestAPI `yaml:"rest-api,omitempty"`
}

type Join struct {
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`
}

type Kubernetes struct {
	Enabled                      *bool  `yaml:"enabled,omitempty"`
	Namespace                    string `yaml:"namespace,omitempty"`
	ServiceName                  string `yaml:"service-name,omitempty"`
	UseNodeNameAsExternalAddress *bool  `yaml:"use-node-name-as-external-address,omitempty"`
	ServicePerPodLabelName       string `yaml:"service-per-pod-label-name,omitempty"`
	ServicePerPodLabelValue      string `yaml:"service-per-pod-label-value,omitempty"`
}

type RestAPI struct {
	Enabled        *bool          `yaml:"enabled,omitempty"`
	EndpointGroups EndpointGroups `yaml:"endpoint-groups,omitempty"`
}

type EndpointGroups struct {
	HealthCheck HealthCheck `yaml:"HEALTH_CHECK,omitempty"`
}

type HealthCheck struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

func (hz Hazelcast) HazelcastConfigForcingRestart() Hazelcast {
	return Hazelcast{
		ClusterName: hz.ClusterName,
		Network: Network{
			Join: Join{
				Kubernetes: Kubernetes{
					ServicePerPodLabelName:       hz.Network.Join.Kubernetes.ServicePerPodLabelName,
					ServicePerPodLabelValue:      hz.Network.Join.Kubernetes.ServicePerPodLabelValue,
					UseNodeNameAsExternalAddress: hz.Network.Join.Kubernetes.UseNodeNameAsExternalAddress,
				},
			},
		},
	}
}
