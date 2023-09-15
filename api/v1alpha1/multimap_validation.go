package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type multimapValidator struct {
	datastructValidator
	name string
}

func (v *multimapValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "MultiMap"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func validateMultiMapSpecCreate(mm *MultiMap) error {
	v := multimapValidator{
		name: mm.Name,
	}
	v.validateDataStructureSpec(&mm.Spec.DataStructureSpec)
	return v.Err()
}

func validateMultiMapSpecUpdate(mm *MultiMap) error {
	v := multimapValidator{
		name: mm.Name,
	}
	v.validateDSSpecUnchanged(mm)
	v.validateDataStructureSpec(&mm.Spec.DataStructureSpec)
	return v.Err()
}
