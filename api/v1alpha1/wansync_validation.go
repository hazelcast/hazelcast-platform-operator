package v1alpha1

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type wansyncValidator struct {
	fieldValidator
}

func NewWanSyncValidator(o client.Object) wansyncValidator {
	return wansyncValidator{NewFieldValidator(o)}
}

func ValidateWanSyncSpec(w *WanSync) error {
	v := NewWanSyncValidator(w)
	v.validateNonUpdatableWanSyncFields(w)
	return v.Err()
}

func (v *wansyncValidator) validateNonUpdatableWanSyncFields(w *WanSync) {
	last, ok := w.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}

	lastSpec := &WanSyncSpec{}
	err := json.Unmarshal([]byte(last), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last WanSync spec for update errors: %w", err))
		return
	}

	if w.Spec.WanReplicationResourceName != lastSpec.WanReplicationResourceName {
		v.Forbidden(Path("spec", "wanReplicationResourceName"), "field cannot be updated")
	}
}
