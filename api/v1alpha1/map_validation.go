package v1alpha1

import (
	"errors"
)

func ValidateNotUpdatableFields(current *MapSpec, last *MapSpec) error {
	if current.Name != last.Name {
		return errors.New("name cannot be updated")
	}
	if *current.BackupCount != *last.BackupCount {
		return errors.New("backupCount cannot be updated")
	}
	if current.AsyncBackupCount != last.AsyncBackupCount {
		return errors.New("asyncBackupCount cannot be updated")
	}
	if !indexConfigSliceEquals(current.Indexes, last.Indexes) {
		return errors.New("indexes cannot be updated")
	}
	if current.PersistenceEnabled != last.PersistenceEnabled {
		return errors.New("persistenceEnabled cannot be updated")
	}
	if current.HazelcastResourceName != last.HazelcastResourceName {
		return errors.New("hazelcastResourceName cannot be updated")
	}
	return nil
}

func indexConfigSliceEquals(a, b []IndexConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if !indexConfigEquals(v, b[i]) {
			return false
		}
	}
	return true
}

func indexConfigEquals(a, b IndexConfig) bool {
	if a.Name != b.Name {
		return false
	}

	if a.Type != b.Type {
		return false
	}

	if !stringSliceEquals(a.Attributes, b.Attributes) {
		return false
	}

	if a.BitmapIndexOptions != b.BitmapIndexOptions {
		return false
	}
	return true
}

func stringSliceEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
