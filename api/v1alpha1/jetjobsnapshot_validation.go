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

func ValidateJetJobSnapshotSpecCreate(jjs *JetJobSnapshot) error {
	var allErrs field.ErrorList
	if jjs.Spec.JetJobResourceName == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("jetJobResourceName"),
			jjs.Spec.JetJobResourceName, "cannot be empty"))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJobSnapshot"}, jjs.Name, allErrs)
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

func ValidateHazelcastLicenseKey(h *Hazelcast) error {
	var allErrs field.ErrorList
	if h.Spec.GetLicenseKeySecretName() == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("licenseKeySecretName"),
			"license key must be set"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "Hazelcast"}, h.Name, allErrs)
}

func ValidateJetJobSnapshotNonUpdatableFields(jjs JetJobSnapshotSpec, oldJjs JetJobSnapshotSpec) []*field.Error {
	var allErrs field.ErrorList

	if !reflect.DeepEqual(jjs, oldJjs) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec"), "field cannot be updated"))
	}

	return allErrs
}
