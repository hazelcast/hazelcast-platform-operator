package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

type queueValidator struct {
	datastructValidator
	name string
}

func (v *queueValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "Queue"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func validateQueueSpecCreate(q *Queue) error {
	v := queueValidator{
		name: q.Name,
	}
	v.validateDataStructureSpec(&q.Spec.DataStructureSpec)
	return v.Err()
}

func validateQueueSpecUpdate(q *Queue) error {
	v := queueValidator{
		name: q.Name,
	}
	v.validateDSSpecUnchanged(q)
	v.validateDataStructureSpec(&q.Spec.DataStructureSpec)
	return v.Err()
}
