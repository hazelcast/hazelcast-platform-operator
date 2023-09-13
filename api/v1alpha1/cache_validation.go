package v1alpha1

import (
	"encoding/json"
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateCacheSpecCurrent(c *Cache, h *Hazelcast) error {
	var allErrs field.ErrorList
	allErrs = appendIfNotNil(allErrs, validateCachePersistence(c, h))
	allErrs = appendIfNotNil(allErrs, validateCacheNativeMemory(c, h))
	if len(allErrs) == 0 {
		return nil
	}

	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Cache"}, c.Name, allErrs)
}

func validateCachePersistence(c *Cache, h *Hazelcast) *field.Error {
	if !c.Spec.PersistenceEnabled {
		return nil
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
	}

	if !lastSpec.Persistence.IsEnabled() {
		return field.Invalid(field.NewPath("spec").Child("persistenceEnabled"), lastSpec.Persistence.IsEnabled(),
			"Persistence must be enabled at Hazelcast")
	}

	return nil
}

func validateCacheNativeMemory(c *Cache, h *Hazelcast) *field.Error {
	if c.Spec.InMemoryFormat != InMemoryFormatNative {
		return nil
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		return field.Invalid(field.NewPath("spec").Child("inMemoryFormat"), lastSpec.NativeMemory.IsEnabled(),
			"Native Memory must be enabled at Hazelcast")
	}
	return nil
}
