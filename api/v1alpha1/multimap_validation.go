package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type multimapValidator struct {
	datastructValidator
}

func NewMultiMapValidator(o client.Object) multimapValidator {
	return multimapValidator{NewDatastructValidator(o)}
}

func validateMultiMapSpecCreate(mm *MultiMap) error {
	v := NewDatastructValidator(mm)
	v.validateDataStructureSpec(&mm.Spec.DataStructureSpec)
	return v.Err()
}

func validateMultiMapSpecUpdate(mm *MultiMap) error {
	v := NewMultiMapValidator(mm)
	v.validateDSSpecUnchanged(mm)
	v.validateDataStructureSpec(&mm.Spec.DataStructureSpec)
	return v.Err()
}
