package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
)

func ValidateManagementCenterSpec(mc *ManagementCenter) error {
	currentErrs := ValidateManagementCenterSpecCurrent(mc)
	updateErrs := ValidateManagementCenterSpecUpdate(mc)
	allErrs := append(currentErrs, updateErrs...)
	if len(allErrs) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "ManagementCenter"}, mc.Name, allErrs)
}

func ValidateManagementCenterSpecCurrent(mc *ManagementCenter) []*field.Error {
	var allErrs field.ErrorList

	if mc.Spec.ExternalConnectivity.Route.IsEnabled() {
		if platform.GetType() != platform.OpenShift {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("externalConnectivity").Child("route"),
				"Route can only be enabled in OpenShift environments."))
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

func ValidateManagementCenterSpecUpdate(mc *ManagementCenter) []*field.Error {
	last, ok := mc.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return nil
	}
	var parsed ManagementCenterSpec

	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		return []*field.Error{field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last ManagementCenter spec for update errors: %w", err))}
	}

	return ValidateNotUpdatableMcPersistenceFields(mc.Spec.Persistence, parsed.Persistence)
}

func ValidateNotUpdatableMcPersistenceFields(current, last McPersistenceConfiguration) []*field.Error {
	var allErrs field.ErrorList

	if reflect.DeepEqual(current.Enabled, last.Enabled) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("enabled"), "field cannot be updated"))
	}
	if current.ExistingVolumeClaimName != last.ExistingVolumeClaimName {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("existingVolumeClaimName"), "field cannot be updated"))
	}
	if current.StorageClass != last.StorageClass {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("storageClass"), "field cannot be updated"))
	}
	if reflect.DeepEqual(current.Size, last.Size) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("size"), "field cannot be updated"))
	}

	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}
