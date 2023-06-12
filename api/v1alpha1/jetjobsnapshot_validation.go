package v1alpha1

import (
	"encoding/json"
	"fmt"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateJetJobSnapshotSpecCreate(jjs *JetJobSnapshot) error {
	return nil
}

func ValidateJetJobSnapshotSpecUpdate(jjs *JetJobSnapshot, _ *JetJobSnapshot) error {
	var allErrs = validateJetJobSnapshotUpdateSpec(jjs)
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJobSnapshot"}, jjs.Name, allErrs)
}

func validateJetJobSnapshotUpdateSpec(jjs *JetJobSnapshot) []*field.Error {
	last, ok := jjs.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return nil
	}
	var parsed JetJobSnapshotSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		return []*field.Error{field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last JetJobSnapshot spec for update errors: %w", err))}
	}
	return ValidateJetJobSnapshotNonUpdatableFields(jjs.Spec, parsed)
}

func ValidateJetJobSnapshotNonUpdatableFields(jjs JetJobSnapshotSpec, oldJjs JetJobSnapshotSpec) []*field.Error {
	var allErrs field.ErrorList

	if jjs.Name != oldJjs.Name {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("name"), "field cannot be updated"))
	}
	if jjs.JetJobResourceName != oldJjs.JetJobResourceName {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("jetJobResourceName"), "field cannot be updated"))
	}
	if jjs.CancelJob != oldJjs.CancelJob {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("cancelJob"), "field cannot be updated"))
	}

	return allErrs
}
