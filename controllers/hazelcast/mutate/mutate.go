package mutate

import (
	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func HazelcastSpec(h *hazelcastv1beta1.Hazelcast) (mutated bool) {
	if h.Spec.LicenseKeySecretName != "" && h.Spec.Repository == n.HazelcastRepo {
		h.Spec.Repository = n.HazelcastEERepo
		mutated = true
	}

	return
}
