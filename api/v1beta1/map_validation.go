package v1beta1

import (
	"errors"
)

func ValidateMapSpec(m *Map, h *Hazelcast) error {
	err := ValidateAppliedPersistence(m.Spec.PersistenceEnabled, h)
	if err != nil {
		return err
	}

	err = ValidateAppliedNativeMemory(m.Spec.InMemoryFormat, h)
	if err != nil {
		return err
	}

	return nil
}

func ValidateNotUpdatableMapFields(current *MapSpec, last *MapSpec) error {
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
	if current.InMemoryFormat != last.InMemoryFormat {
		return errors.New("inMemoryFormat cannot be updated")
	}

	err := isNearCacheUpdated(current, last)
	if err != nil {
		return err
	}

	return nil
}

func isNearCacheUpdated(current *MapSpec, last *MapSpec) error {
	updated := false
	if current.NearCache != nil && last.NearCache != nil {
		if *current.NearCache.InvalidateOnChange != *last.NearCache.InvalidateOnChange ||
			current.NearCache.Name != last.NearCache.Name ||
			*current.NearCache.CacheLocalEntries != *last.NearCache.CacheLocalEntries ||
			current.NearCache.TimeToLiveSeconds != last.NearCache.TimeToLiveSeconds ||
			current.NearCache.MaxIdleSeconds != last.NearCache.MaxIdleSeconds ||
			current.NearCache.InMemoryFormat != last.NearCache.InMemoryFormat {
			updated = true
		}

		if current.NearCache.NearCacheEviction != nil && last.NearCache.NearCacheEviction != nil {
			if current.NearCache.NearCacheEviction.EvictionPolicy != last.NearCache.NearCacheEviction.EvictionPolicy ||
				current.NearCache.NearCacheEviction.Size != last.NearCache.NearCacheEviction.Size ||
				current.NearCache.NearCacheEviction.MaxSizePolicy != last.NearCache.NearCacheEviction.MaxSizePolicy {
				updated = true
			}
		}

		if current.NearCache.NearCacheEviction == nil && last.NearCache.NearCacheEviction != nil {
			updated = true
		}
	}

	if current.NearCache == nil && last.NearCache != nil {
		updated = true
	}

	if updated {
		return errors.New("near cache configuration cannot be updated")
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

	// if both a and b not nil
	if (a.BitmapIndexOptions != nil) && (b.BitmapIndexOptions != nil) {
		return *a.BitmapIndexOptions != *b.BitmapIndexOptions
	}

	// If one of a and b not nil
	if (a.BitmapIndexOptions != nil) || (b.BitmapIndexOptions != nil) {
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
