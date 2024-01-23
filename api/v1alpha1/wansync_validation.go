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
	if w.Spec.Config == nil {
		return
	}
	if w.Spec.Config.TargetClusterName != lastSpec.Config.TargetClusterName {
		v.Forbidden(Path("spec", "config", "targetClusterName"), "field cannot be updated")
	}
	if w.Spec.Config.Endpoints != lastSpec.Config.Endpoints {
		v.Forbidden(Path("spec", "config", "endpoints"), "field cannot be updated")
	}
	if w.Spec.Config.Queue != lastSpec.Config.Queue {
		v.Forbidden(Path("spec", "config", "queue"), "field cannot be updated")
	}
	if w.Spec.Config.Batch != lastSpec.Config.Batch {
		v.Forbidden(Path("spec", "config", "batch"), "field cannot be updated")
	}
	if w.Spec.Config.Acknowledgement != lastSpec.Config.Acknowledgement {
		v.Forbidden(Path("spec", "config", "acknowledgement"), "field cannot be updated")
	}
}
