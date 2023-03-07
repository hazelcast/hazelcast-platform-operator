package hazelcast

import (
	"encoding/json"
	"fmt"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func requireNativeMemory(h *hazelcastv1alpha1.Hazelcast) error {
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name)
	}

	lastSpec := &hazelcastv1alpha1.HazelcastSpec{}
	if err := json.Unmarshal([]byte(s), lastSpec); err != nil {
		return fmt.Errorf("last successful spec for Hazelcast resource %s is not formatted correctly", h.Name)
	}

	if !lastSpec.NativeMemory.IsEnabled() {
		return fmt.Errorf("native memory is not enabled for the Hazelcast resource %s", h.Name)
	}

	return nil
}
