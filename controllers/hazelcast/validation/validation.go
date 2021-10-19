package validation

import (
	"errors"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
)

func ValidateSpec(h *hazelcastv1alpha1.Hazelcast) error {
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
