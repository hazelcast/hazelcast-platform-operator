package v1alpha1

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type topicValidator struct {
	datastructValidator
	name string
}

func (v *topicValidator) Err() error {
	if len(v.fieldValidator) != 0 {
		return kerrors.NewInvalid(
			schema.GroupKind{Group: "hazelcast.com", Kind: "Topic"},
			v.name,
			field.ErrorList(v.fieldValidator),
		)
	}
	return nil
}

func validateTopicSpecCurrent(t *Topic) error {
	v := topicValidator{
		name: t.Name,
	}
	if t.Spec.GlobalOrderingEnabled && t.Spec.MultiThreadingEnabled {
		v.Invalid(Path("spec", "multiThreadingEnabled"), t.Spec.MultiThreadingEnabled, "multi threading can not be enabled when global ordering is used.")
	}
	return v.Err()
}

func validateTopicSpecUpdate(t *Topic) error {
	v := topicValidator{
		name: t.Name,
	}
	v.validateDSSpecUnchanged(t)
	return v.Err()
}
