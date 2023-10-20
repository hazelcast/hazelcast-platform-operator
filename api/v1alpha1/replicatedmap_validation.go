package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

type replicatedmapValidator struct {
	datastructValidator
}

func NewReplicatedMapValidator(o client.Object) replicatedmapValidator {
	return replicatedmapValidator{NewDatastructValidator(o)}
}

func validateReplicatedMapSpecUpdate(rm *ReplicatedMap) error {
	v := NewReplicatedMapValidator(rm)
	v.validateDSSpecUnchanged(rm)
	return v.Err()
}
