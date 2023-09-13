package v1alpha1

import (
	"encoding/json"
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type cacheValidator struct {
	fieldValidator
	name string
}

func (v *cacheValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "Cache"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func ValidateCacheSpecCurrent(c *Cache, h *Hazelcast) error {
	v := cacheValidator{
		name: c.Name,
	}
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
