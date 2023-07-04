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
	allErrs = appendIfNotNil(allErrs, ValidateAppliedPersistence(m.Spec.PersistenceEnabled, h))
	allErrs = appendIfNotNil(allErrs, ValidateAppliedNativeMemory(m.Spec.InMemoryFormat, h))
	if m.Spec.NearCache != nil {
		allErrs = appendIfNotNil(allErrs, ValidateAppliedNativeMemory(m.Spec.NearCache.InMemoryFormat, h))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
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
