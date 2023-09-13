package v1alpha1

import (
	"encoding/json"
	"fmt"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateHotBackupPersistence(h *Hazelcast) *field.Error {
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name))
	}

	lastSpec := &HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return field.InternalError(field.NewPath("spec"), fmt.Errorf("error parsing last Hazelcast spec for update errors: %w", err))
	}

	if !lastSpec.Persistence.IsEnabled() {
		return field.Invalid(field.NewPath("spec").Child("persistenceEnabled"), lastSpec.Persistence.IsEnabled(),
			"Persistence must be enabled at Hazelcast")
	}

	return nil
}
