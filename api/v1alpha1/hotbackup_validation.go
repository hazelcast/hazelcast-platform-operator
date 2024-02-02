package v1alpha1

import (
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type hotbackupValidator struct {
	fieldValidator
}

func NewHotBackupValidator(o client.Object) hotbackupValidator {
	return hotbackupValidator{NewFieldValidator(o)}
}

func ValidateHotBackupPersistence(hb *HotBackup,h *Hazelcast) error {
	v := NewHotBackupValidator(hb)
	v.validateHotBackupPersistence(h)
	return v.Err()
}

func (v *hotbackupValidator) validateHotBackupPersistence(h *Hazelcast) {
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		v.InternalError(Path("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
		return
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		v.InternalError(Path("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
		return
	}

	if !lastSpec.Persistence.IsEnabled() {
		v.Invalid(Path("spec", "persistenceEnabled"), lastSpec.Persistence.IsEnabled(), "Persistence must be enabled at Hazelcast")
		return
	}
}
