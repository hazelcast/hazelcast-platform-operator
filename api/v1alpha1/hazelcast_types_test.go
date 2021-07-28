package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"testing"
)

func TestExposeExternallyConfigurationEnabled(t *testing.T) {
	c := ExposeExternallyConfiguration{}
	assert.False(t, c.IsEnabled(), "Expose externally configuration should not be enabled when not set explicitly")

	c = ExposeExternallyConfiguration{Type: UnisocketExposeExternallyType, DiscoveryServiceType: v1.ServiceTypeLoadBalancer}
	assert.True(t, c.IsEnabled(), "Expose externally configuration should be enabled for Unisocket client configuration")

	c = ExposeExternallyConfiguration{Type: SmartExposeExternallyType, DiscoveryServiceType: v1.LoadBalancerPortsError, MemberAccess: NodeExternalIPMemberAccess}
	assert.True(t, c.IsEnabled(), "Expose externally configuration should be enabled for Smart client configuration")
}
