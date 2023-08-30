package v1alpha1

import (
	"errors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

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
