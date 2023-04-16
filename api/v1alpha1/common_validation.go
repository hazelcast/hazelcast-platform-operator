package v1alpha1

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ValidateAppliedNativeMemory(inMemoryFormat InMemoryFormatType, h *v1beta1.Hazelcast) *field.Error {
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

func ValidateAppliedPersistence(persistenceEnabled bool, h *v1beta1.Hazelcast) *field.Error {
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

func validateDSSpecUnchanged(obj client.Object) error {
	var allErrs field.ErrorList

	ok, err := isDSSpecUnchanged(obj)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(field.NewPath("spec"), err))
		return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: GetKind(obj)}, obj.GetName(), allErrs)
	}
	if !ok {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec"), "cannot be updated"))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: GetKind(obj)}, obj.GetName(), allErrs)
}

func isDSSpecUnchanged(obj client.Object) (bool, error) {
	lastSpec, ok := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return true, nil
	}
	ds, ok := obj.(DataStructure)
	if !ok {
		return false, errors.New("Object is not a data structure")
	}
	newSpec, err := ds.GetSpec()
	if err != nil {
		return false, errors.New("Could not get spec of the data structure")
	}
	if newSpec != lastSpec {
		return false, nil
	}
	return true, nil
}

func appendIfNotNil(errs []*field.Error, err *field.Error) []*field.Error {
	if err == nil {
		return errs
	}
	return append(errs, err)
}
