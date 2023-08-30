package v1alpha1

import (
	"encoding/json"
	"errors"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

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
			"Persistence must be enabled at Hazelcast")
	}

	return nil
}

func validateDSSpecUnchanged(obj client.Object, errorLists ...field.ErrorList) error {
	var allErrs field.ErrorList

	for i := range errorLists {
		allErrs = append(allErrs, errorLists[i]...)
	}

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
	return newSpec == lastSpec, nil
}

func appendIfNotNil(errs []*field.Error, err *field.Error) []*field.Error {
	if err == nil {
		return errs
	}
	return append(errs, err)
}
