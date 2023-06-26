package mutate

import (
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func HazelcastSpec(h *hazelcastv1alpha1.Hazelcast) (mutated bool) {
	if h.Spec.GetLicenseKeySecretName() != "" && h.Spec.Repository == n.HazelcastRepo {
		h.Spec.Repository = n.HazelcastEERepo
		mutated = true
	}
	hazelcastv1alpha1.HzDefaultOptionalToNil(h)

	return
}
