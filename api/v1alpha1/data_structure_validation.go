package v1alpha1

import (
	"errors"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type datastructValidator struct {
	fieldValidator
}

func NewDatastructValidator(o client.Object) datastructValidator {
	return datastructValidator{NewFieldValidator(o)}
}

func (v *datastructValidator) validateDataStructureSpec(ds *DataStructureSpec) {
	if pointer.Int32Deref(ds.BackupCount, 0)+ds.AsyncBackupCount > 6 {
		detail := "the sum of backupCount and asyncBackupCount can't be larger than than 6"
		v.Invalid(Path("spec", "backupCount"), ds.BackupCount, detail)
		v.Invalid(Path("spec", "asyncBackupCount"), ds.AsyncBackupCount, detail)
	}
}

func (v *datastructValidator) validateDSSpecUnchanged(obj client.Object) {
	ok, err := isDSSpecUnchanged(obj)
	if err != nil {
		v.InternalError(Path("spec"), err)
		return
	}
	if !ok {
		v.Forbidden(Path("spec"), "cannot be updated")
	}
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
