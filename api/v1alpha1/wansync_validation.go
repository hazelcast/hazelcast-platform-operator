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
	v.validateNonUpdatableWanFields(w)
	return v.Err()
}

func (v *wansyncValidator) validateNonUpdatableWanFields(w *WanSync) {
	last, ok := w.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}

	lastSpec := &WanSyncSpec{}
	err := json.Unmarshal([]byte(last), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last WanReplication spec for update errors: %w", err))
		return
	}

	if w.Spec.WanReplicationName != lastSpec.WanReplicationName {
		v.Forbidden(Path("spec", "wanReplicationName"), "field cannot be updated")
	}
}
