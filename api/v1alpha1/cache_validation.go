package v1alpha1

import "errors"

func ValidateCacheSpec(c *Cache, h *Hazelcast) error {
	err := ValidateAppliedPersistence(c.Spec.PersistenceEnabled, h)
	if err != nil {
		return err
	}

	err = ValidateAppliedNativeMemory(c.Spec.InMemoryFormat, h)
	if err != nil {
		return err
	}

	return nil
}

func ValidateNotUpdatableCacheFields(current *CacheSpec, last *CacheSpec) error {
	if current.Name != last.Name {
		return errors.New("name cannot be updated")
	}
	if *current.BackupCount != *last.BackupCount {
		return errors.New("backupCount cannot be updated")
	}
	if current.AsyncBackupCount != last.AsyncBackupCount {
		return errors.New("asyncBackupCount cannot be updated")
	}
	if current.PersistenceEnabled != last.PersistenceEnabled {
		return errors.New("persistenceEnabled cannot be updated")
	}
	if current.HazelcastResourceName != last.HazelcastResourceName {
		return errors.New("hazelcastResourceName cannot be updated")
	}
	if current.InMemoryFormat != last.InMemoryFormat {
		return errors.New("inMemoryFormat cannot be updated")
	}
	return nil
}
