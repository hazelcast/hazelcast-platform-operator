package phonehome

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
)

type Metrics struct {
	UID              types.UID
	PardotID         string
	Version          string
	CreatedAt        time.Time
	K8sDistibution   string
	K8sVersion       string
	HazelcastMetrics map[types.UID]*HazelcastMetrics
}
