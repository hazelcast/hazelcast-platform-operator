package phonehome

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
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
	ClientRegistry hzclient.ClientRegistry
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
	OperatorID                    types.UID          `json:"oid"`
	PardotID                      string             `json:"p"`
	Version                       string             `json:"v"`
	Uptime                        int64              `json:"u"` // In milliseconds
	K8sDistibution                string             `json:"kd"`
	K8sVersion                    string             `json:"kv"`
	CreatedClusterCount           int                `json:"ccc"`
	CreatedEnterpriseClusterCount int                `json:"cecc"`
	CreatedMCcount                int                `json:"cmcc"`
	CreatedMemberCount            int                `json:"cmc"`
	ClusterUUIDs                  []string           `json:"cuids"`
	ExposeExternally              ExposeExternally   `json:"xe"`
	Map                           Map                `json:"m"`
	WanReplicationCount           int                `json:"wrc"`
	BackupAndRestore              BackupAndRestore   `json:"br"`
	UserCodeDeployment            UserCodeDeployment `json:"ucd"`
	ExecutorServiceCount          int                `json:"esc"`
	MultiMapCount                 int                `json:"mmc"`
	ReplicatedMapCount            int                `json:"rmc"`
	CronHotBackupCount            int                `json:"chbc"`
	TopicCount                    int                `json:"tc"`
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

type BackupAndRestore struct {
	LocalBackupCount    int `json:"lb"`
	ExternalBackupCount int `json:"eb"`
	PvcCount            int `json:"pvc"`
	HostPathCount       int `json:"hpc"`
	RestoreEnabledCount int `json:"re"`
	GoogleStorage       int `json:"gs"`
	S3                  int `json:"s3"`
	AzureBlobStorage    int `json:"abs"`
}

type UserCodeDeployment struct {
	ClientEnabled int `json:"ce"`
	FromBucket    int `json:"fb"`
	FromConfigMap int `json:"fcm"`
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

	phd.fillHazelcastMetrics(cl, m.ClientRegistry)
	phd.fillMCMetrics(cl)
	phd.fillMapMetrics(cl)
	phd.fillWanReplicationMetrics(cl)
	phd.fillHotBackupMetrics(cl)
	phd.fillMultiMapMetrics(cl)
	phd.fillReplicatedMapMetrics(cl)
	phd.fillCronHotBackupMetrics(cl)
	phd.fillTopicMetrics(cl)
	return phd
}

func upTime(t time.Time) time.Duration {
	now := time.Now()
	return now.Sub(t)
}

func (phm *PhoneHomeData) fillHazelcastMetrics(cl client.Client, hzClientRegistry hzclient.ClientRegistry) {
	createdEnterpriseClusterCount := 0
	createdClusterCount := 0
	createdMemberCount := 0
	executorServiceCount := 0
	clusterUUIDs := []string{}

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
		phm.BackupAndRestore.addUsageMetrics(hz.Spec.Persistence)
		phm.UserCodeDeployment.addUsageMetrics(&hz.Spec.UserCodeDeployment)
		createdMemberCount += int(*hz.Spec.ClusterSize)
		executorServiceCount += len(hz.Spec.ExecutorServices) + len(hz.Spec.DurableExecutorServices) + len(hz.Spec.ScheduledExecutorServices)

		cid, ok := ClusterUUID(hzClientRegistry, hz.Name, hz.Namespace)
		if ok {
			clusterUUIDs = append(clusterUUIDs, cid)
		}
	}
	phm.CreatedClusterCount = createdClusterCount
	phm.CreatedEnterpriseClusterCount = createdEnterpriseClusterCount
	phm.CreatedMemberCount = createdMemberCount
	phm.ExecutorServiceCount = executorServiceCount
	phm.ClusterUUIDs = clusterUUIDs
}

func ClusterUUID(reg hzclient.ClientRegistry, hzName, hzNamespace string) (string, bool) {
	hzcl, ok := reg.Get(types.NamespacedName{Name: hzName, Namespace: hzNamespace})
	if !ok {
		return "", false
	}
	cid := hzcl.ClusterId()
	if cid.Default() {
		return "", false
	}
	return cid.String(), true
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

func (br *BackupAndRestore) addUsageMetrics(p *hazelcastv1alpha1.HazelcastPersistenceConfiguration) {
	if !p.IsEnabled() {
		return
	}
	if p.HostPath != "" {
		br.HostPathCount += 1
	} else if !p.Pvc.IsEmpty() {
		br.PvcCount += 1
	}
	if p.IsRestoreEnabled() {
		br.RestoreEnabledCount += 1
	}
}

func (ucd *UserCodeDeployment) addUsageMetrics(hucd *hazelcastv1alpha1.UserCodeDeploymentConfig) {
	if hucd == nil {
		return
	}
	if hucd.ClientEnabled != nil && *hucd.ClientEnabled {
		ucd.ClientEnabled++
	}
	if hucd.IsBucketEnabled() {
		ucd.FromBucket++
	}
	if hucd.IsConfigMapEnabled() {
		ucd.FromConfigMap++
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

func (phm *PhoneHomeData) fillWanReplicationMetrics(cl client.Client) {
	wrl := &hazelcastv1alpha1.WanReplicationList{}
	err := cl.List(context.Background(), wrl, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil || wrl.Items == nil {
		return
	}
	phm.WanReplicationCount = len(wrl.Items)
}

func (phm *PhoneHomeData) fillHotBackupMetrics(cl client.Client) {
	hbl := &hazelcastv1alpha1.HotBackupList{}
	err := cl.List(context.Background(), hbl, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil {
		return //TODO maybe add retry
	}
	for _, hb := range hbl.Items {
		switch {
		case strings.HasPrefix(hb.Spec.BucketURI, "s3"):
			phm.BackupAndRestore.S3 += 1
		case strings.HasPrefix(hb.Spec.BucketURI, "gs"):
			phm.BackupAndRestore.GoogleStorage += 1
		case strings.HasPrefix(hb.Spec.BucketURI, "azblob"):
			phm.BackupAndRestore.AzureBlobStorage += 1
		}

		if hb.Spec.IsExternal() {
			phm.BackupAndRestore.ExternalBackupCount += 1
		} else {
			phm.BackupAndRestore.LocalBackupCount += 1
		}
	}

}

func (phm *PhoneHomeData) fillMultiMapMetrics(cl client.Client) {
	mml := &hazelcastv1alpha1.MultiMapList{}
	err := cl.List(context.Background(), mml, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil || mml.Items == nil {
		return
	}
	phm.MultiMapCount = len(mml.Items)
}

func (phm *PhoneHomeData) fillCronHotBackupMetrics(cl client.Client) {
	chbl := &hazelcastv1alpha1.CronHotBackupList{}
	err := cl.List(context.Background(), chbl, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil || chbl.Items == nil {
		return
	}
	phm.CronHotBackupCount = len(chbl.Items)
}

func (phm *PhoneHomeData) fillTopicMetrics(cl client.Client) {
	mml := &hazelcastv1alpha1.TopicList{}
	err := cl.List(context.Background(), mml, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil || mml.Items == nil {
		return
	}
	phm.TopicCount = len(mml.Items)
}

func (phm *PhoneHomeData) fillReplicatedMapMetrics(cl client.Client) {
	rml := &hazelcastv1alpha1.ReplicatedMapList{}
	err := cl.List(context.Background(), rml, client.InNamespace(os.Getenv(n.NamespaceEnv)))
	if err != nil || rml.Items == nil {
		return
	}
	phm.ReplicatedMapCount = len(rml.Items)
}
