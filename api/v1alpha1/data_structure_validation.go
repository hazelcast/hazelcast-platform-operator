package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func validateDataStructureSpec(ds *DataStructureSpec) field.ErrorList {
	var errors field.ErrorList

	if pointer.Int32Deref(ds.BackupCount, 0)+ds.AsyncBackupCount > 6 {
		detail := "the sum of backupCount and asyncBackupCount can't be larger than than 6"
		errors = append(errors,
			field.Invalid(field.NewPath("spec").Child("backupCount"), ds.BackupCount, detail),
			field.Invalid(field.NewPath("spec").Child("asyncBackupCount"), ds.AsyncBackupCount, detail),
		)
	}

	if len(errors) == 0 {
		return nil
	}

	return errors
}
