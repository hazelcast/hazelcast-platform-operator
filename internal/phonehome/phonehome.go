package phonehome

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type OperatorInfo struct {
	UID                  types.UID
	PardotID             string
	Version              string
	CreatedAt            time.Time
	K8sDistribution      string
	K8sVersion           string
	Trigger              chan struct{}
	ClientRegistry       hzclient.ClientRegistry
	WatchedNamespaceType util.WatchedNsType
}

func Start(cl client.Client, opInfo *OperatorInfo) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for {
			select {
			case <-opInfo.Trigger:
				// a resource triggered phone home, wait for other possible triggers
				time.Sleep(30 * time.Second)
				// empty other triggers
				for len(opInfo.Trigger) > 0 {
					<-opInfo.Trigger
				}
				PhoneHome(cl, opInfo)
			case <-ticker.C:
				PhoneHome(cl, opInfo)
			}
		}
	}()
}

func PhoneHome(cl client.Client, opInfo *OperatorInfo) {
	phUrl := "http://phonehome.hazelcast.com/pingOp"

	phd := newPhoneHomeData(cl, opInfo)
	jsn, err := json.Marshal(phd)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", phUrl, bytes.NewReader(jsn))
	if err != nil || req == nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 10 * time.Second}
	_, err = c.Do(req)
	if err != nil {
		return
	}
}

type PhoneHomeData struct {
	OperatorID                    types.UID              `json:"oid"`
	PardotID                      string                 `json:"p"`
	Version                       string                 `json:"v"`
	Uptime                        int64                  `json:"u"` // In milliseconds
	K8sDistribution               string                 `json:"kd"`
	K8sVersion                    string                 `json:"kv"`
	WatchedNamespaceType          util.WatchedNsType     `json:"wnt"`
	CreatedClusterCount           int                    `json:"ccc"`
	CreatedEnterpriseClusterCount int                    `json:"cecc"`
	CreatedMCcount                int                    `json:"cmcc"`
	CreatedMemberCount            int                    `json:"cmc"`
	ClusterUUIDs                  []string               `json:"cuids"`
	ExposeExternally              ExposeExternally       `json:"xe"`
	Map                           Map                    `json:"m"`
	Cache                         Cache                  `json:"c"`
	Jet                           Jet                    `json:"jet"`
	WanReplicationCount           int                    `json:"wrc"`
	WanSyncCount                  int                    `json:"wsc"`
	BackupAndRestore              BackupAndRestore       `json:"br"`
	UserCodeDeployment            UserCodeDeployment     `json:"ucd"`
	McExternalConnectivity        McExternalConnectivity `json:"mcec"`
	ExecutorServiceCount          int                    `json:"esc"`
	MultiMapCount                 int                    `json:"mmc"`
	ReplicatedMapCount            int                    `json:"rmc"`
	CronHotBackupCount            int                    `json:"chbc"`
	TopicCount                    int                    `json:"tc"`
	HighAvailabilityMode          []string               `json:"ha"`
	NativeMemoryCount             int                    `json:"nmc"`
	JVMConfigUsage                JVMConfigUsage         `json:"jcu"`
	AdvancedNetwork               AdvancedNetwork        `json:"an"`
	JetEngine                     JetEngine              `json:"je"`
	TLS                           TLS                    `json:"t"`
	SerializationCount            int                    `json:"serc"`
	CustomConfigCount             int                    `json:"ccon"`
	JetJobSnapshotCount           int                    `json:"jjsc"`
	SQLCount                      int                    `json:"sc"`
}

type JVMConfigUsage struct {
	Count           int             `json:"c"`
	AdvancedNetwork AdvancedNetwork `json:"an"`
}

type AdvancedNetwork struct {
	WANEndpointCount int `json:"wec"`
}

type JetEngine struct {
	EnabledCount    int `json:"ec"`
	LosslessRestart int `json:"lr"`
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
	Count             int `json:"c"`
	PersistenceCount  int `json:"pc"`
	MapStoreCount     int `json:"msc"`
	NativeMemoryCount int `json:"nmc"`
	NearCacheCount    int `json:"ncc"`
}

type Cache struct {
	Count             int `json:"c"`
	PersistenceCount  int `json:"pc"`
	NativeMemoryCount int `json:"nmc"`
}

type Jet struct {
	Count int `json:"c"`
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
	FromURL       int `json:"fu"`
}

type McExternalConnectivity struct {
	ServiceTypeClusterIP    int `json:"stci"`
	ServiceTypeNodePort     int `json:"stnp"`
	ServiceTypeLoadBalancer int `json:"stlb"`
	IngressEnabledCount     int `json:"iec"`
	RouteEnabledCount       int `json:"rec"`
}

type TLS struct {
	Count     int `json:"c"`
	MTLSCount int `json:"mc"`
}

func newPhoneHomeData(cl client.Client, opInfo *OperatorInfo) PhoneHomeData {
	phd := PhoneHomeData{
		OperatorID:           opInfo.UID,
		PardotID:             opInfo.PardotID,
		Version:              opInfo.Version,
		Uptime:               upTime(opInfo.CreatedAt).Milliseconds(),
		K8sDistribution:      opInfo.K8sDistribution,
		K8sVersion:           opInfo.K8sVersion,
		WatchedNamespaceType: opInfo.WatchedNamespaceType,
	}

	phd.fillHazelcastMetrics(cl, opInfo.ClientRegistry)
	phd.fillMCMetrics(cl)
	phd.fillMapMetrics(cl)
	phd.fillCacheMetrics(cl)
	phd.fillWanReplicationMetrics(cl)
	phd.fillWanSyncMetrics(cl)
	phd.fillHotBackupMetrics(cl)
	phd.fillMultiMapMetrics(cl)
	phd.fillReplicatedMapMetrics(cl)
	phd.fillCronHotBackupMetrics(cl)
	phd.fillTopicMetrics(cl)
	phd.fillJetMetrics(cl)
	phd.fillSnapshotMetrics(cl)
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
	serializationCount := 0
	customConfigCount := 0
	clusterUUIDs := []string{}
	highAvailabilityModes := []string{}
	nativeMemoryCount := 0
	sqlCount := 0

	hzl := &hazelcastv1alpha1.HazelcastList{}
	err := cl.List(context.Background(), hzl, listOptions()...)
	if err != nil {
		return //TODO maybe add retry
	}

	for _, hz := range hzl.Items {
		if util.IsEnterprise(hz.Spec.Repository) {
			createdEnterpriseClusterCount += 1
		} else {
			createdClusterCount += 1
		}

		if hz.Spec.NativeMemory.IsEnabled() {
			nativeMemoryCount++
		}

		if hz.Spec.Serialization != nil {
			serializationCount++
		}

		if hz.Spec.CustomConfigCmName != "" {
			customConfigCount++
		}

		if hz.Spec.SQL != nil {
			sqlCount++
		}

		phm.ExposeExternally.addUsageMetrics(hz.Spec.ExposeExternally)
		if hz.Spec.AdvancedNetwork != nil {
			phm.AdvancedNetwork.addUsageMetrics(hz.Spec.AdvancedNetwork.WAN)
		}
		phm.BackupAndRestore.addUsageMetrics(hz.Spec.Persistence)
		phm.UserCodeDeployment.addUsageMetrics(hz.Spec.UserCodeDeployment)
		phm.JVMConfigUsage.addUsageMetrics(hz.Spec.JVM)
		phm.JetEngine.addUsageMetrics(hz.Spec.JetEngineConfiguration)
		phm.TLS.addUsageMetrics(hz.Spec.TLS)
		createdMemberCount += int(*hz.Spec.ClusterSize)
		executorServiceCount += len(hz.Spec.ExecutorServices) + len(hz.Spec.DurableExecutorServices) + len(hz.Spec.ScheduledExecutorServices)
		highAvailabilityModes = append(highAvailabilityModes, string(hz.Spec.HighAvailabilityMode))

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
	phm.HighAvailabilityMode = highAvailabilityModes
	phm.NativeMemoryCount = nativeMemoryCount
	phm.SerializationCount = serializationCount
	phm.CustomConfigCount = customConfigCount
	phm.SQLCount = sqlCount
}

func ClusterUUID(reg hzclient.ClientRegistry, hzName, hzNamespace string) (string, bool) {
	hzcl, err := reg.GetOrCreate(context.Background(), types.NamespacedName{Name: hzName, Namespace: hzNamespace})
	if err != nil {
		return "", false
	}
	cid := hzcl.ClusterId()
	if cid.Default() {
		return "", false
	}
	return cid.String(), true
}

func (an *AdvancedNetwork) addUsageMetrics(wc []hazelcastv1alpha1.WANConfig) {
	for _, w := range wc {
		an.WANEndpointCount += int(w.PortCount)
	}
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
	if p.PVC != nil {
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
	if hucd.IsRemoteURLsEnabled() {
		ucd.FromURL++
	}
}

func (j *JVMConfigUsage) addUsageMetrics(jc *hazelcastv1alpha1.JVMConfiguration) {
	if jc == nil {
		return
	}

	if jc.Memory != nil &&
		(jc.Memory.MaxRAMPercentage != nil ||
			jc.Memory.MinRAMPercentage != nil ||
			jc.Memory.InitialRAMPercentage != nil) {
		j.Count += 1
		return
	}

	if jc.GC != nil &&
		(jc.GC.Logging != nil || jc.GC.Collector != nil) {
		j.Count += 1
		return
	}

	if len(jc.Args) > 0 {
		j.Count += 1
		return
	}
}

func (je *JetEngine) addUsageMetrics(jec *hazelcastv1alpha1.JetEngineConfiguration) {
	if !jec.IsEnabled() {
		return
	}

	je.EnabledCount++

	if jec.Instance != nil && jec.Instance.LosslessRestartEnabled {
		je.LosslessRestart++
	}
}

func (t *TLS) addUsageMetrics(tls *hazelcastv1alpha1.TLS) {
	if tls == nil {
		return
	}

	t.Count++

	if tls.MutualAuthentication == hazelcastv1alpha1.MutualAuthenticationRequired {
		t.MTLSCount++
	}
}

func (phm *PhoneHomeData) fillMCMetrics(cl client.Client) {
	createdMCCount := 0
	successfullyCreatedMCCount := 0

	mcl := &hazelcastv1alpha1.ManagementCenterList{}
	err := cl.List(context.Background(), mcl, listOptions()...)
	if err != nil {
		return //TODO maybe add retry
	}

	for _, mc := range mcl.Items {
		createdMCCount += 1
		if mc.Status.Phase == hazelcastv1alpha1.McRunning {
			successfullyCreatedMCCount += 1
		}
		phm.McExternalConnectivity.addUsageMetrics(mc.Spec.ExternalConnectivity)
	}
	phm.CreatedMCcount = createdMCCount
}

func (mec *McExternalConnectivity) addUsageMetrics(ec *hazelcastv1alpha1.ExternalConnectivityConfiguration) {
	if ec.IsEnabled() {
		switch ec.Type {
		case hazelcastv1alpha1.ExternalConnectivityTypeClusterIP:
			mec.ServiceTypeClusterIP++
		case hazelcastv1alpha1.ExternalConnectivityTypeNodePort:
			mec.ServiceTypeNodePort++
		case hazelcastv1alpha1.ExternalConnectivityTypeLoadBalancer:
			mec.ServiceTypeLoadBalancer++
		}
		if ec.Ingress.IsEnabled() {
			mec.IngressEnabledCount++
		}
		if ec.Route.IsEnabled() {
			mec.RouteEnabledCount++
		}
	}
}

func (phm *PhoneHomeData) fillMapMetrics(cl client.Client) {
	createdMapCount := 0
	persistedMapCount := 0
	mapStoreMapCount := 0
	nativeMemoryMapCount := 0
	nearCacheCount := 0

	ml := &hazelcastv1alpha1.MapList{}
	err := cl.List(context.Background(), ml, listOptions()...)
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
		if m.Spec.InMemoryFormat == hazelcastv1alpha1.InMemoryFormatNative {
			nativeMemoryMapCount += 1
		}
		if m.Spec.NearCache != nil {
			nearCacheCount += 1
		}
	}
	phm.Map.Count = createdMapCount
	phm.Map.PersistenceCount = persistedMapCount
	phm.Map.MapStoreCount = mapStoreMapCount
	phm.Map.NativeMemoryCount = nativeMemoryMapCount
	phm.Map.NearCacheCount = nearCacheCount
}

func (phm *PhoneHomeData) fillCacheMetrics(cl client.Client) {
	createdCacheCount := 0
	nativeMemoryCacheCount := 0
	persistedCacheCount := 0

	l := &hazelcastv1alpha1.CacheList{}
	err := cl.List(context.Background(), l, listOptions()...)
	if err != nil {
		return
	}

	for _, c := range l.Items {
		createdCacheCount += 1
		if c.Spec.InMemoryFormat == hazelcastv1alpha1.InMemoryFormatNative {
			nativeMemoryCacheCount += 1
		}
		if c.Spec.PersistenceEnabled {
			persistedCacheCount += 1
		}
	}

	phm.Cache.Count = createdCacheCount
	phm.Cache.PersistenceCount = persistedCacheCount
	phm.Cache.NativeMemoryCount = nativeMemoryCacheCount
}

func (phm *PhoneHomeData) fillWanReplicationMetrics(cl client.Client) {
	wrl := &hazelcastv1alpha1.WanReplicationList{}
	err := cl.List(context.Background(), wrl, listOptions()...)
	if err != nil || wrl.Items == nil {
		return
	}
	phm.WanReplicationCount = len(wrl.Items)
}

func (phm *PhoneHomeData) fillWanSyncMetrics(cl client.Client) {
	wsl := &hazelcastv1alpha1.WanSyncList{}
	err := cl.List(context.Background(), wsl, listOptions()...)
	if err != nil || wsl.Items == nil {
		return
	}
	phm.WanReplicationCount = len(wsl.Items)
}

func (phm *PhoneHomeData) fillHotBackupMetrics(cl client.Client) {
	hbl := &hazelcastv1alpha1.HotBackupList{}
	err := cl.List(context.Background(), hbl, listOptions()...)
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
	err := cl.List(context.Background(), mml, listOptions()...)
	if err != nil || mml.Items == nil {
		return
	}
	phm.MultiMapCount = len(mml.Items)
}

func (phm *PhoneHomeData) fillCronHotBackupMetrics(cl client.Client) {
	chbl := &hazelcastv1alpha1.CronHotBackupList{}
	err := cl.List(context.Background(), chbl, listOptions()...)
	if err != nil || chbl.Items == nil {
		return
	}
	phm.CronHotBackupCount = len(chbl.Items)
}

func (phm *PhoneHomeData) fillTopicMetrics(cl client.Client) {
	mml := &hazelcastv1alpha1.TopicList{}
	err := cl.List(context.Background(), mml, listOptions()...)
	if err != nil || mml.Items == nil {
		return
	}
	phm.TopicCount = len(mml.Items)
}

func (phm *PhoneHomeData) fillReplicatedMapMetrics(cl client.Client) {
	rml := &hazelcastv1alpha1.ReplicatedMapList{}
	err := cl.List(context.Background(), rml, listOptions()...)
	if err != nil || rml.Items == nil {
		return
	}
	phm.ReplicatedMapCount = len(rml.Items)
}

func (phm *PhoneHomeData) fillJetMetrics(cl client.Client) {
	jjl := &hazelcastv1alpha1.JetJobList{}
	err := cl.List(context.Background(), jjl, listOptions()...)
	if err != nil || jjl.Items == nil {
		return
	}
	phm.Jet.Count = len(jjl.Items)
}

func (phm *PhoneHomeData) fillSnapshotMetrics(cl client.Client) {
	jjsl := &hazelcastv1alpha1.JetJobSnapshotList{}
	err := cl.List(context.Background(), jjsl, listOptions()...)
	if err != nil || jjsl.Items == nil {
		return
	}
	phm.JetJobSnapshotCount = len(jjsl.Items)
}

func listOptions() []client.ListOption {
	lo := []client.ListOption{}
	if util.WatchedNamespaceType(util.OperatorNamespace(), util.WatchedNamespaces()) == util.WatchedNsTypeAll {
		// Watching all namespaces, no need to filter
		return lo
	}

	watchedNamespaces := util.WatchedNamespaces()
	for _, watchedNamespace := range watchedNamespaces {
		lo = append(lo, client.InNamespace(watchedNamespace))
	}

	return lo
}
