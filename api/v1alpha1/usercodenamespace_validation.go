package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type userCodeNamespaceValidator struct {
	fieldValidator
}

func NewUCNValidator(o client.Object) userCodeNamespaceValidator {
	return userCodeNamespaceValidator{}
}

func ValidateUCNSpec(u *UserCodeNamespace, h *Hazelcast) error {
	v := NewUCNValidator(u)
	v.validateUCNEnabled(h)
	return v.Err()
}

func (v userCodeNamespaceValidator) validateUCNEnabled(h *Hazelcast) {
	if !h.Spec.UserCodeNamespaces.IsEnabled() {
		v.Required(Path("spec", "userCodeNamespace"), "should be enabled in Hazelcast")
	}
}
