package phonehome

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type Metrics struct {
	UID              types.UID
	PardotID         string
	Version          string
	CreatedAt        time.Time
	K8sDistibution   string
	K8sVersion       string
	HazelcastMetrics map[types.UID]*HazelcastMetrics
	MCMetrics        map[types.UID]*MCMetrics
}

type PhoneHomeData struct {
	OperatorID                    types.UID        `json:"oid"`
	PardotID                      string           `json:"p"`
	Version                       string           `json:"v"`
	Uptime                        int64            `json:"u"` // In milliseconds
	K8sDistibution                string           `json:"kd"`
	K8sVersion                    string           `json:"kv"`
	CreatedClusterCount           int              `json:"ccc"`
	CreatedEnterpriseClusterCount int              `json:"cecc"`
	CreatedMCcount                int              `json:"cmcc"`
	AverageClusterCreationLatency int64            `json:"accl,omitempty"`  // In milliseconds
	AverageMCCreationLatency      int64            `json:"amccl,omitempty"` // In milliseconds
	CreatedMemberCount            int              `json:"cmc"`
	ExposeExternally              ExposeExternally `json:"xe"`
}

type ExposeExternally struct {
	Unisocket                int `json:"u"`
	Smart                    int `json:"s"`
	DiscoveryLoadBalancer    int `json:"dlb"`
	DiscoveryNodePort        int `json:"dnp"`
	MemberNodePortExternalIP int `json:"mpei"`
	MemberNodePortNodeName   int `json:"mnpnn"`
	MemberLoadBalancer       int `json:"mlb"`
}

func (xe *ExposeExternally) addUsageMetrics(e hazelcastv1alpha1.ExposeExternallyConfiguration) {
	if !e.IsEnabled() {
		return
	}

	if e.DiscoveryK8ServiceType() == v1.ServiceTypeLoadBalancer {
		xe.DiscoveryLoadBalancer += 1
	} else {
		xe.DiscoveryNodePort += 1
	}

	if !e.IsSmart() {
		xe.Unisocket += 1
		return
	}

	xe.Smart += 1

	switch ma := e.MemberAccess; ma {
	case hazelcastv1alpha1.MemberAccessLoadBalancer:
		xe.MemberLoadBalancer += 1
	case hazelcastv1alpha1.MemberAccessNodePortNodeName:
		xe.MemberNodePortNodeName += 1
	default:
		xe.MemberNodePortExternalIP += 1
	}
}

func Start(m *Metrics) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			PhoneHome(m)
		}
	}()
}

func PhoneHome(m *Metrics) {
	phUrl := "http://phonehome.hazelcast.com/pingOp"

	phd := fillPhoneHomeData(m)
	jsn, err := json.Marshal(phd)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", phUrl, bytes.NewReader(jsn))
	if err != nil || req == nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	_, err = client.Do(req)
	if err != nil {
		return
	}
}

func fillPhoneHomeData(m *Metrics) PhoneHomeData {
	phd := PhoneHomeData{
		OperatorID:     m.UID,
		PardotID:       m.PardotID,
		Version:        m.Version,
		Uptime:         upTime(m.CreatedAt).Milliseconds(),
		K8sDistibution: m.K8sDistibution,
		K8sVersion:     m.K8sVersion,
	}

	phd.fillHazelcastMetrics(m.HazelcastMetrics)
	phd.fillMCMetrics(m.MCMetrics)
	return phd
}

func upTime(t time.Time) time.Duration {
	now := time.Now()
	return now.Sub(t)
}

func (phm *PhoneHomeData) fillHazelcastMetrics(m map[types.UID]*HazelcastMetrics) {
	createdEnterpriseClusterCount := 0
	createdClusterCount := 0
	createdMemberCount := 0
	totalClusterCreationLatency := int64(0)
	averageClusterCreationLatency := int64(0)
	successfullyCreatedClusterCount := 0

	for _, v := range m {
		if v.Enterprise {
			createdEnterpriseClusterCount += 1
		} else {
			createdClusterCount += 1
		}

		phm.ExposeExternally.addUsageMetrics(v.ExposeExternally)
		createdMemberCount += int(v.MemberCount)

		dur := v.creationLatency().Milliseconds()
		// if the creation latency is 0, it won't affect the average cluster creation latency.
		if dur == int64(0) {
			continue
		}
		successfullyCreatedClusterCount += 1
		totalClusterCreationLatency += dur

	}
	if successfullyCreatedClusterCount != 0 {
		averageClusterCreationLatency = totalClusterCreationLatency / int64(successfullyCreatedClusterCount)
	}

	phm.CreatedClusterCount = createdClusterCount
	phm.CreatedEnterpriseClusterCount = createdEnterpriseClusterCount
	phm.AverageClusterCreationLatency = averageClusterCreationLatency
	phm.CreatedMemberCount = createdMemberCount
}

func (phm *PhoneHomeData) fillMCMetrics(m map[types.UID]*MCMetrics) {
	createdMCCount := 0
	totalMCCreationLatency := int64(0)
	averageMCCreationLatency := int64(0)
	successfullyCreatedMCCount := 0

	for _, v := range m {
		createdMCCount += 1

		dur := v.creationLatency().Milliseconds()
		// if the creation latency is 0, it won't affect the average cluster creation latency.
		if dur == int64(0) {
			continue
		}
		successfullyCreatedMCCount += 1
		totalMCCreationLatency += dur
	}
	if successfullyCreatedMCCount != 0 {
		averageMCCreationLatency = totalMCCreationLatency / int64(successfullyCreatedMCCount)
	}

	phm.AverageMCCreationLatency = averageMCCreationLatency
	phm.CreatedMCcount = createdMCCount
}

func CallPhoneHome(metrics *Metrics) {
	go func() {
		PhoneHome(metrics)
	}()
}
