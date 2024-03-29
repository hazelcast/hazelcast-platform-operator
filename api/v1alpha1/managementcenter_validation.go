package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/kubeclient"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
)

type managementCenterValidator struct {
	fieldValidator
}

func NewManagementCenterValidator(o client.Object) managementCenterValidator {
	return managementCenterValidator{NewFieldValidator(o)}
}

func ValidateManagementCenterSpec(mc *ManagementCenter) error {
	v := NewManagementCenterValidator(mc)
	v.validateSpecCurrent(mc)
	v.validateSpecUpdate(mc)
	return v.Err()
}

func (v *managementCenterValidator) validateSpecCurrent(mc *ManagementCenter) {
	if mc.Spec.ExternalConnectivity.Route.IsEnabled() {
		if platform.GetType() != platform.OpenShift {
			v.Forbidden(Path("spec", "externalConnectivity", "route"), "Route can only be enabled in OpenShift environments.")
		}
	}
	for i := range mc.Spec.HazelcastClusters {
		v.validateClusterConfigTLS(&mc.Spec.HazelcastClusters[i], mc.Namespace)
	}
	if mc.Spec.SecurityProviders != nil {
		v.validateSecurityProviders(mc.Spec.SecurityProviders, mc.Namespace)
	}
	if mc.Spec.ExternalConnectivity.IsEnabled() {
		v.validateExternalConnectivity(mc.Spec.ExternalConnectivity)
	}
}

func (v *managementCenterValidator) validateSpecUpdate(mc *ManagementCenter) {
	last, ok := mc.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}
	var parsed ManagementCenterSpec

	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last ManagementCenter spec for update errors: %w", err))
		return
	}

	v.ValidateNotUpdatableMcPersistenceFields(mc.Spec.Persistence, parsed.Persistence)
}

func (v *managementCenterValidator) ValidateNotUpdatableMcPersistenceFields(current, last *MCPersistenceConfiguration) {
	if current.IsEnabled() != last.IsEnabled() {
		v.Forbidden(Path("spec", "persistence", "enabled"), "field cannot be updated")
	}
	if current == nil || last == nil {
		return
	}
	if current.ExistingVolumeClaimName != last.ExistingVolumeClaimName {
		v.Forbidden(Path("spec", "persistence", "existingVolumeClaimName"), "field cannot be updated")
	}
	if current.StorageClass != last.StorageClass {
		v.Forbidden(Path("spec", "persistence", "storageClass"), "field cannot be updated")
	}
	if !reflect.DeepEqual(current.Size, last.Size) {
		v.Forbidden(Path("spec", "persistence", "size"), "field cannot be updated")
	}
}

func (v *managementCenterValidator) validateClusterConfigTLS(config *HazelcastClusterConfig, namespace string) {
	// skip validation if TLS is not set
	if config.TLS == nil {
		return
	}

	p := Path("spec", "hazelcastClusters", "tls", "secretName")

	// if user skipped validation secretName can be empty
	if config.TLS.SecretName == "" {
		v.NotFound(p, "Management Center Cluster config TLS Secret name is empty")
		return
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
		v.NotFound(p, "Management Center Cluster config TLS Secret not found")
		return
	}
}

func (v *managementCenterValidator) validateSecurityProviders(config *SecurityProviders, namespace string) {
	if config.LDAP == nil {
		return
	}

	p := Path("spec", "securityProviders", "ldap", "credentialsSecretName")

	if config.LDAP.CredentialsSecretName == "" {
		v.NotFound(p, "Management Center LDAP credentials Secret name is empty")
		return
	}

	// check if secret exists
	secretName := types.NamespacedName{
		Name:      config.LDAP.CredentialsSecretName,
		Namespace: namespace,
	}

	var secret corev1.Secret
	err := kubeclient.Get(context.Background(), secretName, &secret)
	if kerrors.IsNotFound(err) {
		// we care only about not found error
		v.NotFound(p, "Management Center LDAP credentials Secret not found")
		return
	}
}

func (v *managementCenterValidator) validateExternalConnectivity(config *ExternalConnectivityConfiguration) {
	if config.Ingress != nil {
		if !path.IsAbs(config.Ingress.Path) {
			v.Invalid(Path("spec", "externalConnectivity", "ingress", "path"), config.Ingress.Path, "must be an absolute path")
		}
	}
}
