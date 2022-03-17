package phonehome

import (
	"time"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/util"
)

type HazelcastMetrics struct {
	ClusterID              string
	Enterprise             bool
	CreatedAt              time.Time
	SuccessfullyDeployedAt time.Time
	MemberCount            int32
	LastUpdateTimestamp    time.Time
	ExposeExternally       *hazelcastv1alpha1.ExposeExternallyConfiguration
}

func (hm *HazelcastMetrics) creationLatency() time.Duration {
	if !hm.SuccessfullyDeployedAt.IsZero() {
		return hm.SuccessfullyDeployedAt.Sub(hm.CreatedAt)
	}
	// If the cluster is not successfuly deployed yet or it was created by previous operator runs
	return time.Duration(0)
}

func (hm *HazelcastMetrics) FillInitial(h *hazelcastv1alpha1.Hazelcast) {
	if hm.CreatedAt.IsZero() {
		hm.CreatedAt = time.Now()
	}

	hm.Enterprise = util.IsEnterprise(h.Spec.Repository)
	hm.ExposeExternally = h.Spec.ExposeExternally
	hm.MemberCount = *h.Spec.ClusterSize
}

func (hm *HazelcastMetrics) FillAfterDeployment(h *hazelcastv1alpha1.Hazelcast) bool {
	firstDeployment := false
	// If operator is restarted, existing clusters won't be labeled as firstDeployment
	if _, ok := h.Annotations[n.LastSuccessfulSpecAnnotation]; hm.SuccessfullyDeployedAt.IsZero() && !ok {
		hm.SuccessfullyDeployedAt = time.Now()
		firstDeployment = true
	}

	return firstDeployment
}
