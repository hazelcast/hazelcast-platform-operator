package v1alpha1

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateTopicSpecCurrent(t *Topic) error {
	var allErrs field.ErrorList

	if t.Spec.GlobalOrderingEnabled && t.Spec.MultiThreadingEnabled {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("multiThreadingEnabled"), t.Spec.MultiThreadingEnabled,
			"multi threading can not be enabled when global ordering is used."))
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Topic"}, t.Name, allErrs)
}
