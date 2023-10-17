package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type mapValidator struct {
	datastructValidator
	name string
}

func (v *mapValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "Map"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func ValidateMapSpecCreate(m *Map) error {
	v := mapValidator{
		name: m.Name,
	}
	v.validateDataStructureSpec(&m.Spec.DataStructureSpec)
	v.validateMapTieredStore(m)
	return v.Err()
}

func ValidateMapSpecUpdate(m *Map) error {
	v := mapValidator{
		name: m.Name,
	}
	v.validateMapSpecUpdate(m)
	v.validateDataStructureSpec(&m.Spec.DataStructureSpec)
	return v.Err()
}

func ValidateMapSpec(m *Map, h *Hazelcast) error {
	v := mapValidator{
		name: m.Name,
	}
	v.validateMapSpecCurrent(m, h)
	v.validateMapSpecUpdate(m)
	v.validateDataStructureSpec(&m.Spec.DataStructureSpec)
	return v.Err()
}

func (v *mapValidator) validateMapSpecCurrent(m *Map, h *Hazelcast) {
	v.validateMapPersistence(m, h)
	v.validateMapNativeMemory(m, h)
	v.validateNearCacheMemory(m, h)
}

func (v *mapValidator) validateMapPersistence(m *Map, h *Hazelcast) {
	if !m.Spec.PersistenceEnabled {
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

func (v *mapValidator) validateNearCacheMemory(m *Map, h *Hazelcast) {
	if m.Spec.NearCache == nil {
		return
	}

	if m.Spec.NearCache.InMemoryFormat != InMemoryFormatNative {
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

func (v *mapValidator) validateMapNativeMemory(m *Map, h *Hazelcast) {
	if m.Spec.InMemoryFormat != InMemoryFormatNative {
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

func (v *mapValidator) validateMapTieredStore(m *Map) {
	if m.Spec.TieredStore == nil {
		return
	}
	if m.Spec.InMemoryFormat != InMemoryFormatNative {
		v.Invalid(Path("spec", "inMemoryFormat"), m.Spec.InMemoryFormat, "In-memory format of the map must be NATIVE to enable the tiered Storage")
	}
	if len(m.Spec.Indexes) != 0 {
		v.Invalid(Path("spec", "indexes"), m.Spec.Indexes, "Indexes can not be created on maps that have Tiered Storage enabled")
	}
}

func (v *mapValidator) validateMapSpecUpdate(m *Map) {
	last, ok := m.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}
	var parsed MapSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Map spec for update errors: %w", err))
		return
	}

	v.validateNotUpdatableMapFields(&m.Spec, &parsed)
}

func (v *mapValidator) validateNotUpdatableMapFields(current *MapSpec, last *MapSpec) {
	if current.Name != last.Name {
		v.Forbidden(Path("spec", "name"), "field cannot be updated")
	}
	if *current.BackupCount != *last.BackupCount {
		v.Forbidden(Path("spec", "backupCount"), "field cannot be updated")
	}
	if current.AsyncBackupCount != last.AsyncBackupCount {
		v.Forbidden(Path("spec", "asyncBackupCount"), "field cannot be updated")
	}
	if !indexConfigSliceEquals(current.Indexes, last.Indexes) {
		v.Forbidden(Path("spec", "indexes"), "field cannot be updated")
	}
	if current.PersistenceEnabled != last.PersistenceEnabled {
		v.Forbidden(Path("spec", "persistenceEnabled"), "field cannot be updated")
	}
	if current.HazelcastResourceName != last.HazelcastResourceName {
		v.Forbidden(Path("spec", "hazelcastResourceName"), "field cannot be updated")
	}
	if current.InMemoryFormat != last.InMemoryFormat {
		v.Forbidden(Path("spec", "inMemoryFormat"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.EventJournal, last.EventJournal) {
		v.Forbidden(Path("spec", "eventJournal"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.NearCache, last.NearCache) {
		v.Forbidden(Path("spec", "nearCache"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.TieredStore, last.TieredStore) {
		v.Forbidden(Path("spec", "tieredStore"), "field cannot be updated")
	}
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
