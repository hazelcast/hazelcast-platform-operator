package v1alpha1

import (
	"encoding/json"
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type wanreplicationValidator struct {
	fieldValidator
}

func NewWanReplicationValidator(o client.Object) wanreplicationValidator {
	return wanreplicationValidator{NewFieldValidator(o)}
}

func ValidateWanReplicationSpec(w *WanReplication) error {
	v := NewWanReplicationValidator(w)
	v.validateNonUpdatableWanFields(w)
	return v.Err()
}

func (v *wanreplicationValidator) validateNonUpdatableWanFields(w *WanReplication) {
	last, ok := w.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return
	}

	lastSpec := &WanReplicationSpec{}
	err := json.Unmarshal([]byte(last), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last WanReplication spec for update errors: %w", err))
		return
	}

	if w.Spec.TargetClusterName != lastSpec.TargetClusterName {
		v.Forbidden(Path("spec", "targetClusterName"), "field cannot be updated")
	}
	if w.Spec.Endpoints != lastSpec.Endpoints {
		v.Forbidden(Path("spec", "endpoints"), "field cannot be updated")
	}
	if w.Spec.Queue != lastSpec.Queue {
		v.Forbidden(Path("spec", "queue"), "field cannot be updated")
	}
	if w.Spec.Batch != lastSpec.Batch {
		v.Forbidden(Path("spec", "batch"), "field cannot be updated")
	}
	if w.Spec.Acknowledgement != lastSpec.Acknowledgement {
		v.Forbidden(Path("spec", "acknowledgement"), "field cannot be updated")
	}
}
