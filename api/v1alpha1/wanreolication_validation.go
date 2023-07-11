package v1alpha1

import (
	"encoding/json"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func ValidateWanReplicationSpec(w *WanReplication) error {
	errors := validateNonUpdatableWanFields(w)
	if len(errors) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "WanReplication"}, r.Name, errors)
}

func validateNonUpdatableWanFields(w *WanReplication) field.ErrorList {
	var allErrs field.ErrorList
	last, ok := w.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return allErrs
	}
	lastSpec := &WanReplicationSpec{}
	err := json.Unmarshal([]byte(last), lastSpec)
	if err != nil {
		return field.ErrorList{field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last WanReplication spec for update errors: %w", err))}
	}

	if w.Spec.TargetClusterName != lastSpec.TargetClusterName {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("targetClusterName"), "field cannot be updated"))
	}
	if w.Spec.Endpoints != lastSpec.Endpoints {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("endpoints"), "field cannot be updated"))
	}
	if w.Spec.Queue != lastSpec.Queue {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("queue"), "field cannot be updated"))
	}
	if w.Spec.Batch != lastSpec.Batch {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("batch"), "field cannot be updated"))
	}
	if w.Spec.Acknowledgement != lastSpec.Acknowledgement {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("acknowledgement"), "field cannot be updated"))
	}
	return allErrs
}
