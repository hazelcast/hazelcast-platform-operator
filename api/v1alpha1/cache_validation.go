package v1alpha1

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type cacheValidator struct {
	datastructValidator
}

func NewCacheValidator(o client.Object) cacheValidator {
	return cacheValidator{NewDatastructValidator(o)}
}

func validateCacheSpecCreate(c *Cache) error {
	v := NewCacheValidator(c)
	v.validateDataStructureSpec(&c.Spec.DataStructureSpec)
	return v.Err()
}

func validateCacheSpecUpdate(c *Cache) error {
	v := NewCacheValidator(c)
	v.validateDSSpecUnchanged(c)
	v.validateDataStructureSpec(&c.Spec.DataStructureSpec)
	return v.Err()
}

func ValidateCacheSpecCurrent(c *Cache, h *Hazelcast) error {
	v := NewCacheValidator(c)
	v.validateCachePersistence(c, h)
	v.validateCacheNativeMemory(c, h)
	return v.Err()
}

func (v *cacheValidator) validateCachePersistence(c *Cache, h *Hazelcast) {
	if !c.Spec.PersistenceEnabled {
		return
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		v.InternalError(Path("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
		return
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
		return
	}

	if !lastSpec.Persistence.IsEnabled() {
		v.Invalid(Path("spec", "persistenceEnabled"), lastSpec.Persistence.IsEnabled(), "Persistence must be enabled at Hazelcast")
		return
	}
}

func (v *cacheValidator) validateCacheNativeMemory(c *Cache, h *Hazelcast) {
	if c.Spec.InMemoryFormat != InMemoryFormatNative {
		return
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		v.InternalError(Path("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
		return
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
		return
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		v.Invalid(Path("spec", "inMemoryFormat"), lastSpec.NativeMemory.IsEnabled(), "Native Memory must be enabled at Hazelcast")
		return
	}
}
