package mutate

import (
	"testing"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func Test_HazelcastSpec_SetDefaultValues(t *testing.T) {
	h := &hazelcastv1alpha1.Hazelcast{
		Spec: hazelcastv1alpha1.HazelcastSpec{
			Repository:                 n.HazelcastRepo,
			DeprecatedLicenseKeySecret: "my-license-key",
		},
	}
	if mutated := HazelcastSpec(h); !mutated {
		t.Errorf("Expecting Hazelcast Spec to be mutated to default values")
	}
	if h.Spec.Repository != n.HazelcastEERepo {
		t.Errorf("Expecting repository to be set to Enterprise")
	}
	if h.Spec.DeprecatedLicenseKeySecret != "" || h.Spec.LicenseKeySecretName != "my-license-key" {
		t.Errorf("Expecting DeprecatedLicenseKeySecret field to be set to the LicenseKeySecretName field")
	}
}
