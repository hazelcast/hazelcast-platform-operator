package validation

import (
	"errors"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

func ValidateSpec(h *hazelcastv1alpha1.Hazelcast) error {
	if err := validateExposeExternally(h); err != nil {
		return err
	}

	if err := validateLicense(h); err != nil {
		return err
	}

	return nil
}

func validateExposeExternally(h *hazelcastv1alpha1.Hazelcast) error {
	ee := h.Spec.ExposeExternally
	if ee == nil {
		return nil
	}

	if ee.Type == hazelcastv1alpha1.ExposeExternallyTypeUnisocket && ee.MemberAccess != "" {
		return errors.New("when exposeExternally.type is set to \"Unisocket\", exposeExternally.memberAccess must not be set")
	}

	return nil
}

func validateLicense(h *hazelcastv1alpha1.Hazelcast) error {
	if util.IsEnterprise(h.Spec.Repository) && len(h.Spec.LicenseKeySecret) == 0 {
		return errors.New("when Hazelcast Enterprise is deployed, licenseKeySecret must be set")
	}
	return nil
}
