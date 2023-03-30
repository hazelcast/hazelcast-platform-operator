package v1alpha1

import (
	"encoding/json"
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateAppliedNativeMemory(inMemoryFormat InMemoryFormatType, h *Hazelcast) *field.Error {
	if inMemoryFormat != InMemoryFormatNative {
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

func ValidateAppliedPersistence(persistenceEnabled bool, h *Hazelcast) *field.Error {
	if !persistenceEnabled {
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
			"data structure persistence must match with Hazelcast persistence")
	}

	return nil
}

func appendIfNotNil(errs []*field.Error, err *field.Error) []*field.Error {
	if err == nil {
		return errs
	}
	return append(errs, err)
}
