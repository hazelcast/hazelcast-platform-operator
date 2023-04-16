package v1alpha1

import (
	"github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateCacheSpecCurrent(c *Cache, h *v1beta1.Hazelcast) error {
	var allErrs field.ErrorList
	allErrs = appendIfNotNil(allErrs, ValidateAppliedPersistence(c.Spec.PersistenceEnabled, h))
	allErrs = appendIfNotNil(allErrs, ValidateAppliedNativeMemory(c.Spec.InMemoryFormat, h))
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Cache"}, c.Name, allErrs)
}
