package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

type topicValidator struct {
	datastructValidator
}

func NewTopicValidator(o client.Object) topicValidator {
	return topicValidator{NewDatastructValidator(o)}
}

func validateTopicSpecCurrent(t *Topic) error {
	v := NewTopicValidator(t)
	if t.Spec.GlobalOrderingEnabled && t.Spec.MultiThreadingEnabled {
		v.Invalid(Path("spec", "multiThreadingEnabled"), t.Spec.MultiThreadingEnabled, "multi threading can not be enabled when global ordering is used.")
	}
	return v.Err()
}

func validateTopicSpecUpdate(t *Topic) error {
	v := NewTopicValidator(t)
	v.validateDSSpecUnchanged(t)
	return v.Err()
}
