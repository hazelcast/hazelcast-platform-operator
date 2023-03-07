package v1alpha1

import "errors"

func ValidateNotUpdatableCacheFields(current *CacheSpec, last *CacheSpec) error {
	if current.InMemoryFormat != last.InMemoryFormat {
		return errors.New("inMemoryFormat cannot be updated")
	}
	return nil
}
