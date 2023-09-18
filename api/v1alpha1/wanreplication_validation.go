package v1alpha1

import (
	"encoding/json"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type wanreplicationValidator struct {
	fieldValidator
	name string
}

func (v *wanreplicationValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "WanReplication"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func ValidateWanReplicationSpec(w *WanReplication) error {
	v := wanreplicationValidator{
		name: w.Name,
	}
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
