package v1beta1

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateJetConfiguration(h *Hazelcast) error {
	var allErrs field.ErrorList
	if h.Spec.JetEngineConfiguration.Enabled != nil && !*h.Spec.JetEngineConfiguration.Enabled {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("jet").Child("enabled"),
			h.Spec.JetEngineConfiguration.Enabled, "jet engine must be enabled"))
	}
	if !h.Spec.JetEngineConfiguration.ResourceUploadEnabled {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("jet").Child("resourceUploadEnabled"),
			h.Spec.JetEngineConfiguration.ResourceUploadEnabled, "jet engine resource upload must be enabled"))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Hazelcast"}, h.Name, allErrs)
}
