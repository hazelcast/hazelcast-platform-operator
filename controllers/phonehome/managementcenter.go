package phonehome

import (
	"time"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

type MCMetrics struct {
	CreatedAt              time.Time
	SuccessfullyDeployedAt time.Time
}

func (mcm *MCMetrics) creationLatency() time.Duration {
	if mcm.SuccessfullyDeployedAt != (time.Time{}) {
		return mcm.SuccessfullyDeployedAt.Sub(mcm.CreatedAt)
	}
	// If the MC is not successfuly deployed yet or it was created by previous operator runs
	return time.Duration(0)
}

func (mcm *MCMetrics) FillInitial(h *hazelcastv1alpha1.ManagementCenter) {
	if _, ok := h.Annotations[n.LastSuccessfulSpecAnnotation]; mcm.CreatedAt == (time.Time{}) && !ok {
		mcm.CreatedAt = time.Now()
	}
}

func (mcm *MCMetrics) FillAfterDeployment(h *hazelcastv1alpha1.ManagementCenter) bool {
	firstDeployment := false
	// If operator is restarted, existing MCs won't be labeled as firstDeployment
	if _, ok := h.Annotations[n.LastSuccessfulSpecAnnotation]; mcm.SuccessfullyDeployedAt == (time.Time{}) && !ok {
		mcm.SuccessfullyDeployedAt = time.Now()
		firstDeployment = true

	}
	return firstDeployment
}
