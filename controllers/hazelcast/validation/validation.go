package validation

import (
	"errors"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
)

func ValidateInitalSpec(h *hazelcastv1alpha1.Hazelcast) error {
	return validateSpec(h)
}

func ValidateUpdatedSpec(h *hazelcastv1alpha1.Hazelcast, oldSpec *hazelcastv1alpha1.HazelcastSpec) error {
	// We can check for fields whose states we want to trace between updates.
	return validateSpec(h)
}

func validateSpec(h *hazelcastv1alpha1.Hazelcast) error {
	err := validateExposeExternally(h)
	if err != nil {
		return err
	}

	return nil
}

func validateExposeExternally(h *hazelcastv1alpha1.Hazelcast) error {
	ee := h.Spec.ExposeExternally

	if ee.Type == hazelcastv1alpha1.ExposeExternallyTypeUnisocket && ee.MemberAccess != "" {
		return errors.New("when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set")
	}

	return nil
}
