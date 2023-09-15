package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type replicatedmapValidator struct {
	datastructValidator
	name string
}

func (v *replicatedmapValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "ReplicatedMap"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func validateReplicatedMapSpecUpdate(rm *ReplicatedMap) error {
	v := replicatedmapValidator{
		name: rm.Name,
	}
	v.validateDSSpecUnchanged(rm)
	return v.Err()
}
