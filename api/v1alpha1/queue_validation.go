package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

type queueValidator struct {
	datastructValidator
}

func NewQueueValidator(o client.Object) queueValidator {
	return queueValidator{NewDatastructValidator(o)}
}

func validateQueueSpecCreate(q *Queue) error {
	v := NewQueueValidator(q)
	v.validateDataStructureSpec(&q.Spec.DataStructureSpec)
	return v.Err()
}

func validateQueueSpecUpdate(q *Queue) error {
	v := NewQueueValidator(q)
	v.validateDSSpecUnchanged(q)
	v.validateDataStructureSpec(&q.Spec.DataStructureSpec)
	return v.Err()
}
