package v1beta1

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestExternalConnectivityConfigurationType(t *testing.T) {
	tests := []struct {
		name string
		conf ExternalConnectivityConfiguration
		want v1.ServiceType
	}{
		{
			name: "Empty configuration",
			conf: ExternalConnectivityConfiguration{},
			want: v1.ServiceTypeLoadBalancer,
		},
		{
			name: "ClusterIP service type configuration",
			conf: ExternalConnectivityConfiguration{
				Type: ExternalConnectivityTypeClusterIP,
			},
			want: v1.ServiceTypeClusterIP,
		},
		{
			name: "NodePort service type configuration",
			conf: ExternalConnectivityConfiguration{
				Type: ExternalConnectivityTypeNodePort,
			},
			want: v1.ServiceTypeNodePort,
		},
		{
			name: "LoadBalancer service type configuration",
			conf: ExternalConnectivityConfiguration{
				Type: ExternalConnectivityTypeLoadBalancer,
			},
			want: v1.ServiceTypeLoadBalancer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.ManagementCenterServiceType(); got != tt.want {
				t.Errorf("ManagementCenterServiceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExternalConnectivityConfigurationIngress(t *testing.T) {
	tests := []struct {
		name string
		conf ExternalConnectivityConfiguration
		want bool
	}{
		{
			name: "Default configuration",
			conf: ExternalConnectivityConfiguration{},
			want: false,
		},
		{
			name: "Ingress enabled configuration",
			conf: ExternalConnectivityConfiguration{
				Ingress: &ExternalConnectivityIngress{},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.Ingress.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersistenceConfigurationIsEnabled(t *testing.T) {
	tests := []struct {
		name string
		conf PersistenceConfiguration
		want bool
	}{
		{
			name: "Default configuration",
			conf: PersistenceConfiguration{},
			want: false,
		},
		{
			name: "Enabled configuration",
			conf: PersistenceConfiguration{
				Enabled: pointer.Bool(true),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
