package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"testing"
)

func TestExposeExternallyConfigurationIsEnabled(t *testing.T) {
	tests := []struct {
		name string
		conf ExposeExternallyConfiguration
		want bool
	}{
		{
			name: "Empty configuration",
			conf: ExposeExternallyConfiguration{},
			want: false,
		},
		{
			name: "Unisocket configuration",
			conf: ExposeExternallyConfiguration{
				Type:                 ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: v1.ServiceTypeLoadBalancer,
			},
			want: true,
		},
		{
			name: "Smart configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeSmart,
				MemberAccess: MemberAccessNodePortExternalIP,
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

func TestTestExposeExternallyConfigurationIsSmart(t *testing.T) {
	tests := []struct {
		name string
		conf ExposeExternallyConfiguration
		want bool
	}{
		{
			name: "Empty configuration",
			conf: ExposeExternallyConfiguration{},
			want: false,
		},
		{
			name: "Unisocket configuration",
			conf: ExposeExternallyConfiguration{
				Type:                 ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: v1.ServiceTypeLoadBalancer,
			},
			want: false,
		},
		{
			name: "Smart configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeSmart,
				MemberAccess: MemberAccessNodePortExternalIP,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.IsSmart(); got != tt.want {
				t.Errorf("IsSmart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTestExposeExternallyConfigurationUsesNodeName(t *testing.T) {
	tests := []struct {
		name string
		conf ExposeExternallyConfiguration
		want bool
	}{
		{
			name: "Empty configuration",
			conf: ExposeExternallyConfiguration{},
			want: false,
		},
		{
			name: "NodePortExternalIP member access configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeSmart,
				MemberAccess: MemberAccessNodePortExternalIP,
			},
			want: false,
		},
		{
			name: "NodePortNodeName member access configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeSmart,
				MemberAccess: MemberAccessNodePortNodeName,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.UsesNodeName(); got != tt.want {
				t.Errorf("UsesNodeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTestExposeExternallyConfigurationDiscoveryK8ServiceType(t *testing.T) {
	tests := []struct {
		name string
		conf ExposeExternallyConfiguration
		want v1.ServiceType
	}{
		{
			name: "Empty configuration",
			conf: ExposeExternallyConfiguration{},
			want: v1.ServiceTypeLoadBalancer,
		},
		{
			name: "NodePort discovery service configuration",
			conf: ExposeExternallyConfiguration{
				Type:                 ExposeExternallyTypeUnisocket,
				DiscoveryServiceType: v1.ServiceTypeNodePort,
			},
			want: v1.ServiceTypeNodePort,
		},
		{
			name: "LoadBalancer discovery service configuration",
			conf: ExposeExternallyConfiguration{
				Type:                 ExposeExternallyTypeSmart,
				DiscoveryServiceType: v1.ServiceTypeLoadBalancer,
			},
			want: v1.ServiceTypeLoadBalancer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.DiscoveryK8ServiceType(); got != tt.want {
				t.Errorf("DiscoveryK8ServiceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTestExposeExternallyConfigurationMemberAccessServiceType(t *testing.T) {
	tests := []struct {
		name string
		conf ExposeExternallyConfiguration
		want v1.ServiceType
	}{
		{
			name: "Empty configuration",
			conf: ExposeExternallyConfiguration{},
			want: v1.ServiceTypeNodePort,
		},
		{
			name: "NodePortExternalIP member access configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeUnisocket,
				MemberAccess: MemberAccessNodePortExternalIP,
			},
			want: v1.ServiceTypeNodePort,
		},
		{
			name: "NodePortNodeName member access configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeUnisocket,
				MemberAccess: MemberAccessNodePortNodeName,
			},
			want: v1.ServiceTypeNodePort,
		},
		{
			name: "LoadBalancer member access configuration",
			conf: ExposeExternallyConfiguration{
				Type:         ExposeExternallyTypeUnisocket,
				MemberAccess: MemberAccessLoadBalancer,
			},
			want: v1.ServiceTypeLoadBalancer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.conf.MemberAccessServiceType(); got != tt.want {
				t.Errorf("MemberAccessServiceType() = %v, want %v", got, tt.want)
			}
		})
	}
}
