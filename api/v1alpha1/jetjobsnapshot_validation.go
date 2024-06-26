package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type jetJobSnapshotValidator struct {
	fieldValidator
}

func NewJetJobSnapshotValidator(o client.Object) jetJobSnapshotValidator {
	return jetJobSnapshotValidator{NewFieldValidator(o)}
}

func ValidateJetJobSnapshotSpecUpdate(jjs *JetJobSnapshot, _ *JetJobSnapshot) error {
	v := NewJetJobSnapshotValidator(jjs)
	v.validateJetJobSnapshotUpdateSpec(jjs)
	return v.Err()
}

func (v *jetJobSnapshotValidator) validateJetJobSnapshotUpdateSpec(jjs *JetJobSnapshot) {
	last, ok := jjs.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}
	var parsed JetJobSnapshotSpec
	if err := json.Unmarshal([]byte(last), &parsed); err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last JetJobSnapshot spec for update errors: %w", err))
		return
	}

	v.validateJetJobSnapshotNonUpdatableFields(jjs.Spec, parsed)
}

func ValidateHazelcastLicenseKey(h *Hazelcast) error {
	v := newHazelcastValidator(h)
	if h.Spec.GetLicenseKeySecretName() == "" {
		v.Required(Path("spec", "licenseKeySecretName"), "license key must be set")
	}
	return v.Err()
}

func (v *jetJobSnapshotValidator) validateJetJobSnapshotNonUpdatableFields(jjs JetJobSnapshotSpec, oldJjs JetJobSnapshotSpec) {
	if !reflect.DeepEqual(jjs, oldJjs) {
		v.Forbidden(Path("spec"), "field cannot be updated")
	}
}
