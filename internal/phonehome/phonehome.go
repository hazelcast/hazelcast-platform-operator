package phonehome

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type Metrics struct {
	UID            types.UID
	PardotID       string
	Version        string
	CreatedAt      time.Time
	K8sDistibution string
	K8sVersion     string
	Trigger        chan struct{}
}

func Start(cl client.Client, m *Metrics) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-m.Trigger:
				// a resource triggered phone home, wait for other possible triggers
				time.Sleep(30 * time.Second)
				// empty other triggers
				for len(m.Trigger) > 0 {
					<-m.Trigger
				}
				PhoneHome(cl, m)
			case <-ticker.C:
				PhoneHome(cl, m)
			}
		}
	}()
}

func PhoneHome(cl client.Client, m *Metrics) {
	phUrl := "http://phonehome.hazelcast.com/pingOp"

	phd := newPhoneHomeData(cl, m)
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
	CreatedMemberCount            int              `json:"cmc"`
	ExposeExternally              ExposeExternally `json:"xe"`
	Map                           Map              `json:"m"`
}

type ExposeExternally struct {
	Unisocket                int `json:"u"`
	Smart                    int `json:"s"`
	DiscoveryLoadBalancer    int `json:"dlb"`
	DiscoveryNodePort        int `json:"dnp"`
	MemberNodePortExternalIP int `json:"mnpei"`
	MemberNodePortNodeName   int `json:"mnpnn"`
	MemberLoadBalancer       int `json:"mlb"`
}

type Map struct {
	Count            int `json:"c"`
	PersistenceCount int `json:"pc"`
	MapStoreCount    int `json:"msc"`
}

func newPhoneHomeData(cl client.Client, m *Metrics) PhoneHomeData {
	phd := PhoneHomeData{
		OperatorID:     m.UID,
		PardotID:       m.PardotID,
		Version:        m.Version,
		Uptime:         upTime(m.CreatedAt).Milliseconds(),
		K8sDistibution: m.K8sDistibution,
		K8sVersion:     m.K8sVersion,
	}

	phd.fillHazelcastMetrics(cl)
	phd.fillMCMetrics(cl)
	phd.fillMapMetrics(cl)
	return phd
}

func upTime(t time.Time) time.Duration {
	now := time.Now()
	return now.Sub(t)
}

func (phm *PhoneHomeData) fillHazelcastMetrics(cl client.Client) {
	createdEnterpriseClusterCount := 0
	createdClusterCount := 0
	createdMemberCount := 0

	hzl := &hazelcastv1alpha1.HazelcastList{}
	err := cl.List(context.Background(), hzl, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil {
		return //TODO maybe add retry
	}

	for _, hz := range hzl.Items {
		if util.IsEnterprise(hz.Spec.Repository) {
			createdEnterpriseClusterCount += 1
		} else {
			createdClusterCount += 1
		}

		phm.ExposeExternally.addUsageMetrics(hz.Spec.ExposeExternally)
		createdMemberCount += int(*hz.Spec.ClusterSize)
	}
	phm.CreatedClusterCount = createdClusterCount
	phm.CreatedEnterpriseClusterCount = createdEnterpriseClusterCount
	phm.CreatedMemberCount = createdMemberCount
}

func (xe *ExposeExternally) addUsageMetrics(e *hazelcastv1alpha1.ExposeExternallyConfiguration) {
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

func (phm *PhoneHomeData) fillMCMetrics(cl client.Client) {
	createdMCCount := 0
	successfullyCreatedMCCount := 0

	mcl := &hazelcastv1alpha1.ManagementCenterList{}
	err := cl.List(context.Background(), mcl, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil {
		return //TODO maybe add retry
	}

	for _, mc := range mcl.Items {
		createdMCCount += 1
		if mc.Status.Phase == hazelcastv1alpha1.Running {
			successfullyCreatedMCCount += 1
		}
	}
	phm.CreatedMCcount = createdMCCount
}

func (phm *PhoneHomeData) fillMapMetrics(cl client.Client) {
	createdMapCount := 0
	persistedMapCount := 0
	mapStoreMapCount := 0

	ml := &hazelcastv1alpha1.MapList{}
	err := cl.List(context.Background(), ml, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil {
		return //TODO maybe add retry
	}

	for _, m := range ml.Items {
		createdMapCount += 1
		if m.Spec.PersistenceEnabled {
			persistedMapCount += 1
		}
		if m.Spec.MapStore != nil {
			mapStoreMapCount += 1
		}
	}
	phm.Map.Count = createdMapCount
	phm.Map.PersistenceCount = persistedMapCount
	phm.Map.MapStoreCount = mapStoreMapCount
}
