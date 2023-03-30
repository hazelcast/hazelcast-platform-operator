package v1alpha1

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateCacheSpecUpdate(c *Cache) error {
	err := field.Forbidden(field.NewPath("spec"),
		"cannot be updated")
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Cache"}, c.Name, field.ErrorList{err})
}

func ValidateCacheSpecCurrent(c *Cache, h *Hazelcast) error {
	var allErrs field.ErrorList
	allErrs = appendIfNotNil(allErrs, ValidateAppliedPersistence(c.Spec.PersistenceEnabled, h))
	allErrs = appendIfNotNil(allErrs, ValidateAppliedNativeMemory(c.Spec.InMemoryFormat, h))
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Cache"}, c.Name, allErrs)
}
