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

func ValidateMapSpecCreate(m *Map) error {
	errors := validateDataStructureSpec(&m.Spec.DataStructureSpec)
	if len(errors) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Map"}, m.Name, errors)
}

func ValidateMapSpecUpdate(m *Map) error {
	var errors field.ErrorList
	errors = append(errors, validateMapSpecUpdate(m)...)
	errors = append(errors, validateDataStructureSpec(&m.Spec.DataStructureSpec)...)
	if len(errors) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Map"}, m.Name, errors)
}

func ValidateMapSpec(m *Map, h *Hazelcast) error {
	var errors field.ErrorList
	errors = append(errors, validateMapSpecCurrent(m, h)...)
	errors = append(errors, validateMapSpecUpdate(m)...)
	errors = append(errors, validateDataStructureSpec(&m.Spec.DataStructureSpec)...)
	if len(errors) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Map"}, m.Name, errors)
}

func validateMapSpecCurrent(m *Map, h *Hazelcast) field.ErrorList {
	var allErrs field.ErrorList
	allErrs = appendIfNotNil(allErrs, validateMapPersistence(m, h))
	allErrs = appendIfNotNil(allErrs, validateMapNativeMemory(m, h))
	allErrs = appendIfNotNil(allErrs, validateNearCacheMemory(m, h))
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func validateMapPersistence(m *Map, h *Hazelcast) *field.Error {
	if !m.Spec.PersistenceEnabled {
		return nil
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
	}

	if !lastSpec.Persistence.IsEnabled() {
		return field.Invalid(field.NewPath("spec").Child("persistenceEnabled"), lastSpec.Persistence.IsEnabled(),
			"Persistence must be enabled at Hazelcast")
	}

	return nil
}

func validateNearCacheMemory(m *Map, h *Hazelcast) *field.Error {
	if m.Spec.NearCache == nil {
		return nil
	}

	if m.Spec.NearCache.InMemoryFormat != InMemoryFormatNative {
		return nil
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		return field.Invalid(field.NewPath("spec").Child("inMemoryFormat"), lastSpec.NativeMemory.IsEnabled(),
			"Native Memory must be enabled at Hazelcast")
	}
	return nil
}

func validateMapNativeMemory(m *Map, h *Hazelcast) *field.Error {
	if m.Spec.InMemoryFormat != InMemoryFormatNative {
		return nil
	}

	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		return field.Invalid(field.NewPath("spec").Child("inMemoryFormat"), lastSpec.NativeMemory.IsEnabled(),
			"Native Memory must be enabled at Hazelcast")
	}
	return nil
}

func validateMapSpecUpdate(m *Map) field.ErrorList {
	last, ok := m.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return nil
	}
	var parsed MapSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		return field.ErrorList{field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Map spec for update errors: %w", err))}
	}

	return ValidateNotUpdatableMapFields(&m.Spec, &parsed)
}

func ValidateNotUpdatableMapFields(current *MapSpec, last *MapSpec) field.ErrorList {
	var allErrs field.ErrorList

	if current.Name != last.Name {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("name"), "field cannot be updated"))
	}
	if *current.BackupCount != *last.BackupCount {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("backupCount"), "field cannot be updated"))
	}
	if current.AsyncBackupCount != last.AsyncBackupCount {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("asyncBackupCount"), "field cannot be updated"))
	}
	if !indexConfigSliceEquals(current.Indexes, last.Indexes) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("indexes"), "field cannot be updated"))
	}
	if current.PersistenceEnabled != last.PersistenceEnabled {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistenceEnabled"), "field cannot be updated"))
	}
	if current.HazelcastResourceName != last.HazelcastResourceName {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("hazelcastResourceName"), "field cannot be updated"))
	}
	if current.InMemoryFormat != last.InMemoryFormat {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("inMemoryFormat"), "field cannot be updated"))
	}
	if !reflect.DeepEqual(current.EventJournal, last.EventJournal) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("eventJournal"), "field cannot be updated"))
	}

	if !reflect.DeepEqual(current.NearCache, last.NearCache) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("nearCache"), "field cannot be updated"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
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
