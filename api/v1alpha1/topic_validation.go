package v1alpha1

import (
	"errors"
)

func ValidateTopicSpec(t *Topic) error {
	if t.Spec.GlobalOrderingEnabled && t.Spec.MultiThreadingEnabled {
		return errors.New("multi threading can not be enabled when global ordering is used.")
	}
	return nil
}
