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

type jetJobSnapshotValidator struct {
	fieldValidator
	name string
}

func (v *jetJobSnapshotValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "JetJobSnapshot"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func ValidateJetJobSnapshotSpecUpdate(jjs *JetJobSnapshot, _ *JetJobSnapshot) error {
	v := jetJobSnapshotValidator{
		name: jjs.Name,
	}
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
	v := hazelcastValidator{
		name: h.Name,
	}
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
