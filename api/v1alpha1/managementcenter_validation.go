package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
)

func ValidateManagementCenterSpec(mc *ManagementCenter) error {
	var errors field.ErrorList
	errors = append(errors, ValidateManagementCenterSpecCurrent(mc)...)
	errors = append(errors, ValidateManagementCenterSpecUpdate(mc)...)
	for i := range mc.Spec.HazelcastClusters {
		err := validateClusterConfigTLS(&mc.Spec.HazelcastClusters[i], mc.Namespace)
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return kerrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "ManagementCenter"}, mc.Name, errors)
}

func ValidateManagementCenterSpecCurrent(mc *ManagementCenter) field.ErrorList {
	var allErrs field.ErrorList

	if mc.Spec.ExternalConnectivity.Route.IsEnabled() {
		if platform.GetType() != platform.OpenShift {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("externalConnectivity").Child("route"),
				"Route can only be enabled in OpenShift environments."))
		}
	}
	return allErrs
}

func ValidateManagementCenterSpecUpdate(mc *ManagementCenter) field.ErrorList {
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

func ValidateNotUpdatableMcPersistenceFields(current, last MCPersistenceConfiguration) field.ErrorList {
	var allErrs field.ErrorList

	if !reflect.DeepEqual(current.Enabled, last.Enabled) {
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
	if !reflect.DeepEqual(current.Size, last.Size) {
		allErrs = append(allErrs,
			field.Forbidden(field.NewPath("spec").Child("persistence").Child("size"), "field cannot be updated"))
	}
	return allErrs
}

func validateClusterConfigTLS(config *HazelcastClusterConfig, namespace string) *field.Error {
	// skip validation if TLS is not set
	if config.TLS == nil {
		return nil
	}

	p := field.NewPath("spec").Child("hazelcastClusters").Child("tls").Child("secretName")

	// if user skipped validation secretName can be empty
	if config.TLS.SecretName == "" {
		return field.NotFound(p, "Management Center Cluster config TLS Secret name is empty")
	}

	// check if secret exists
	secretName := types.NamespacedName{
		Name:      config.TLS.SecretName,
		Namespace: namespace,
	}

	var secret corev1.Secret
	err := kubeclient.Get(context.Background(), secretName, &secret)
	if kerrors.IsNotFound(err) {
		// we care only about not found error
		return field.NotFound(p, "Management Center Cluster config TLS Secret not found")
	}

	return nil
}
