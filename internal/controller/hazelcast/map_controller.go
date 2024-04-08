package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	proto "github.com/hazelcast/hazelcast-go-client"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// MapReconciler reconciles a Map object
type MapReconciler struct {
	client.Client
	Log              logr.Logger
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewMapReconciler(c client.Client, log logr.Logger, pht chan struct{}, cs hzclient.ClientRegistry) *MapReconciler {
	return &MapReconciler{
		Client:           c,
		Log:              log,
		phoneHomeTrigger: pht,
		clientRegistry:   cs,
	}
}

const retryAfterForMap = 5 * time.Second

//+kubebuilder:rbac:groups=hazelcast.com,resources=maps,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=maps/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=maps/finalizers,verbs=update,namespace=watched

func (r *MapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-map", req.NamespacedName)

	m := &hazelcastv1alpha1.Map{}
	err := r.Client.Get(ctx, req.NamespacedName, m)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Map resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Map: %w", err)
	}

	err = util.AddFinalizer(ctx, r.Client, m, logger)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
			withMapFailedState(err.Error()))
	}

	if m.GetDeletionTimestamp() != nil {
		updateMapStatus(ctx, r.Client, m, recoptions.Empty(), withMapState(hazelcastv1alpha1.MapTerminating)) //nolint:errcheck
		err = r.executeFinalizer(ctx, m)
		if err != nil {
			return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
				withMapState(hazelcastv1alpha1.MapTerminating),
				withMapMessage(err.Error()))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: m.Spec.HazelcastResourceName}, h)
	if err != nil {
		err = fmt.Errorf("could not create/update Map config: Hazelcast resource not found: %w", err)
		return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
			withMapFailedState(err.Error()))
	}
	if h.Status.Phase != hazelcastv1alpha1.Running {
		err = errors.NewServiceUnavailable("Hazelcast CR is not ready")
		return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
			withMapFailedState(err.Error()))
	}

	err = hazelcastv1alpha1.ValidateMapSpec(m, h)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
			withMapFailedState(fmt.Sprintf("error validating new Spec: %s", err)))
	}

	s, createdBefore := m.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if createdBefore {
		ms, err := json.Marshal(m.Spec)
		if err != nil {
			err = fmt.Errorf("error marshaling Map as JSON: %w", err)
			return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
				withMapFailedState(err.Error()))
		}
		if s == string(ms) {
			logger.Info("Map Config was already applied.", "name", m.Name, "namespace", m.Namespace)
			return updateMapStatus(ctx, r.Client, m, recoptions.Empty(), withMapState(hazelcastv1alpha1.MapSuccess))
		}
	}

	cl, err := getHazelcastClient(ctx, r.clientRegistry, m.Spec.HazelcastResourceName, m.Namespace)
	if err != nil {
		if errors.IsInternalError(err) {
			return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
				withMapFailedState(err.Error()))
		}
		return updateMapStatus(ctx, r.Client, m, recoptions.RetryAfter(retryAfterForMap),
			withMapState(hazelcastv1alpha1.MapPending),
			withMapMessage(err.Error()))
	}

	if m.Status.State != hazelcastv1alpha1.MapPersisting {
		requeue, err := updateMapStatus(ctx, r.Client, m, recoptions.Empty(),
			withMapState(hazelcastv1alpha1.MapPending),
			withMapMessage("Applying new map configuration."))
		if err != nil {
			return requeue, err
		}
	}

	ms, err := r.ReconcileMapConfig(ctx, m, h, cl, createdBefore)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, recoptions.RetryAfter(retryAfterForMap),
			withMapState(hazelcastv1alpha1.MapPending),
			withMapMessage(err.Error()),
			withMapMemberStatuses(ms))
	}

	requeue, err := updateMapStatus(ctx, r.Client, m, recoptions.RetryAfter(1*time.Second),
		withMapState(hazelcastv1alpha1.MapPersisting),
		withMapMessage("Persisting the applied map config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateMapConfigPersistence(ctx, h, m)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, recoptions.Error(err),
			withMapFailedState(err.Error()))
	}

	if !persisted {
		return updateMapStatus(ctx, r.Client, m, recoptions.RetryAfter(1*time.Second),
			withMapState(hazelcastv1alpha1.MapPersisting),
			withMapMessage("Waiting for Map Config to be persisted."))
	}

	if util.IsPhoneHomeEnabled() && !recoptions.IsSuccessfullyApplied(m) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, m)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	return updateMapStatus(ctx, r.Client, m, recoptions.Empty(),
		withMapState(hazelcastv1alpha1.MapSuccess),
		withMapMessage(""),
		withMapMemberStatuses{})
}

func (r *MapReconciler) executeFinalizer(ctx context.Context, m *hazelcastv1alpha1.Map) error {
	if !controllerutil.ContainsFinalizer(m, n.Finalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(m, n.Finalizer)
	err := r.Update(ctx, m)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func getHazelcastClient(ctx context.Context, cs hzclient.ClientRegistry, hzName, hzNamespace string) (hzclient.Client, error) {
	hzcl, err := cs.GetOrCreate(ctx, types.NamespacedName{Name: hzName, Namespace: hzNamespace})
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("cannot connect to the cluster for %s", hzName))
	}
	if !hzcl.Running() {
		return nil, fmt.Errorf("trying to connect to the cluster %s", hzName)
	}

	return hzcl, nil
}

func (r *MapReconciler) ReconcileMapConfig(
	ctx context.Context,
	m *hazelcastv1alpha1.Map,
	hz *hazelcastv1alpha1.Hazelcast,
	cl hzclient.Client,
	createdBefore bool,
) (map[string]hazelcastv1alpha1.MapConfigState, error) {
	var req *proto.ClientMessage
	if createdBefore {
		req = codec.EncodeMCUpdateMapConfigRequest(
			m.MapName(),
			m.Spec.TimeToLiveSeconds,
			m.Spec.MaxIdleSeconds,
			hazelcastv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy],
			false,
			m.Spec.Eviction.MaxSize,
			hazelcastv1alpha1.EncodeMaxSizePolicy[m.Spec.Eviction.MaxSizePolicy],
		)
	} else {
		mapInput := codecTypes.DefaultAddMapConfigInput()
		err := fillAddMapConfigInput(ctx, r.Client, mapInput, hz, m)
		if err != nil {
			return nil, err
		}
		req = codec.EncodeDynamicConfigAddMapConfigRequest(mapInput)
	}

	memberStatuses := map[string]hazelcastv1alpha1.MapConfigState{}
	var failedMembers strings.Builder
	for _, member := range cl.OrderedMembers() {
		if status, ok := m.Status.MemberStatuses[member.UUID.String()]; ok && status == hazelcastv1alpha1.MapSuccess {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MapSuccess
			continue
		}
		_, err := cl.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MapFailed
			failedMembers.WriteString(member.UUID.String() + ", ")
			r.Log.Error(err, "Failed with member")
			continue
		}
		memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MapSuccess
	}
	errString := failedMembers.String()
	if errString != "" {
		return memberStatuses, fmt.Errorf("error creating/updating the Map config %s for members %s", m.MapName(), errString[:len(errString)-2])
	}

	return memberStatuses, nil
}

func fillAddMapConfigInput(ctx context.Context, c client.Client, mapInput *codecTypes.AddMapConfigInput, hz *hazelcastv1alpha1.Hazelcast, m *hazelcastv1alpha1.Map) error {
	mapInput.Name = m.MapName()

	ms := m.Spec
	mapInput.BackupCount = *ms.BackupCount
	mapInput.AsyncBackupCount = ms.AsyncBackupCount
	mapInput.TimeToLiveSeconds = ms.TimeToLiveSeconds
	mapInput.MaxIdleSeconds = ms.MaxIdleSeconds

	mapInput.EvictionConfig.EvictionPolicy = string(ms.Eviction.EvictionPolicy)
	mapInput.EvictionConfig.Size = ms.Eviction.MaxSize
	mapInput.EvictionConfig.MaxSizePolicy = string(ms.Eviction.MaxSizePolicy)

	mapInput.IndexConfigs = copyIndexes(ms.Indexes)
	mapInput.AttributeConfigs = copyAttributes(ms.Attributes)
	mapInput.HotRestartConfig.Enabled = ms.PersistenceEnabled
	mapInput.WanReplicationRef = defaultWanReplicationRefCodec(hz, m)
	mapInput.InMemoryFormat = string(ms.InMemoryFormat)
	mapInput.UserCodeNamespace = ms.UserCodeNamespace
	if ms.MerkleTree != nil {
		mapInput.MerkleTreeConfig = codecTypes.MerkleTreeConfig{
			Enabled:    true,
			Depth:      ms.MerkleTree.Depth,
			EnabledSet: true,
		}
	}

	if ms.MapStore != nil {
		props, err := getMapStoreProperties(ctx, c, ms.MapStore.PropertiesSecretName, hz.Namespace)
		if err != nil {
			return err
		}
		// TODO: Temporary solution for https://github.com/hazelcast/hazelcast/issues/21799
		if len(props) == 0 {
			props = map[string]string{"no_empty_props_allowed": ""}
		}
		mapInput.MapStoreConfig.Enabled = true
		mapInput.MapStoreConfig.ClassName = ms.MapStore.ClassName
		mapInput.MapStoreConfig.WriteCoalescing = true
		if ms.MapStore.WriteCoealescing != nil {
			mapInput.MapStoreConfig.WriteCoalescing = *ms.MapStore.WriteCoealescing
		}
		mapInput.MapStoreConfig.WriteDelaySeconds = ms.MapStore.WriteDelaySeconds
		mapInput.MapStoreConfig.WriteBatchSize = ms.MapStore.WriteBatchSize
		mapInput.MapStoreConfig.Properties = props
		mapInput.MapStoreConfig.InitialLoadMode = string(ms.MapStore.InitialMode)
	}
	if len(m.Spec.EntryListeners) != 0 {
		lch := make([]codecTypes.ListenerConfigHolder, 0, len(m.Spec.EntryListeners))
		for _, el := range m.Spec.EntryListeners {
			lch = append(lch, codecTypes.ListenerConfigHolder{
				ClassName:    el.ClassName,
				IncludeValue: el.GetIncludedValue(),
				Local:        el.Local,
				ListenerType: 2, //For EntryListenerConfig
			})
		}
		mapInput.ListenerConfigs = lch
	}

	if m.Spec.NearCache != nil {
		mapInput.NearCacheConfig.Name = ms.NearCache.Name
		mapInput.NearCacheConfig.InMemoryFormat = string(ms.NearCache.InMemoryFormat)
		mapInput.NearCacheConfig.InvalidateOnChange = *ms.NearCache.InvalidateOnChange
		mapInput.NearCacheConfig.TimeToLiveSeconds = int32(ms.NearCache.TimeToLiveSeconds)
		mapInput.NearCacheConfig.MaxIdleSeconds = int32(ms.NearCache.MaxIdleSeconds)
		mapInput.NearCacheConfig.CacheLocalEntries = *ms.NearCache.CacheLocalEntries
		//TODO: We should remove this assignment after the fix: https://github.com/hazelcast/hazelcast/issues/23978
		mapInput.NearCacheConfig.LocalUpdatePolicy = "INVALIDATE"
		//TODO: We should remove this assignment after the fix: https://github.com/hazelcast/hazelcast/issues/23979
		mapInput.NearCacheConfig.PreloaderConfig =
			codecTypes.NearCachePreloaderConfig{
				Enabled:                  false,
				Directory:                "",
				StoreInitialDelaySeconds: 600,
				StoreIntervalSeconds:     600,
			}

		mapInput.NearCacheConfig.EvictionConfigHolder = codecTypes.EvictionConfigHolder{
			EvictionPolicy: string(ms.NearCache.NearCacheEviction.EvictionPolicy),
			MaxSizePolicy:  string(ms.NearCache.NearCacheEviction.MaxSizePolicy),
			Size:           int32(ms.NearCache.NearCacheEviction.Size),
		}

	}

	if m.Spec.EventJournal != nil {
		mapInput.EventJournalConfig.Enabled = true
		mapInput.EventJournalConfig.Capacity = m.Spec.EventJournal.Capacity
		mapInput.EventJournalConfig.TimeToLiveSeconds = m.Spec.EventJournal.TimeToLiveSeconds
	}

	if ms.TieredStore != nil {
		mapInput.TieredStoreConfig.Enabled = true
		mapInput.TieredStoreConfig.MemoryTierConfig.Capacity = codecTypes.Capacity{
			Value: ms.TieredStore.MemoryCapacity.Value(),
			Unit:  "BYTES",
		}
		mapInput.TieredStoreConfig.DiskTierConfig.Enabled = true
		mapInput.TieredStoreConfig.DiskTierConfig.DeviceName = ms.TieredStore.DiskDeviceName
	}
	return nil
}

func defaultWanReplicationRefCodec(hz *hazelcastv1alpha1.Hazelcast, m *hazelcastv1alpha1.Map) codecTypes.WanReplicationRef {
	if !util.IsEnterprise(hz.Spec.Repository) {
		return codecTypes.WanReplicationRef{}
	}

	return codecTypes.WanReplicationRef{
		Name:                 defaultWanReplicationRefName(m),
		MergePolicyClassName: n.DefaultMergePolicyClassName,
		Filters:              []string{},
		RepublishingEnabled:  true,
	}
}

func defaultWanReplicationRefName(m *hazelcastv1alpha1.Map) string {
	return m.MapName() + "-default"
}

func copyIndexes(idx []hazelcastv1alpha1.IndexConfig) []codecTypes.IndexConfig {
	ics := make([]codecTypes.IndexConfig, len(idx))

	for i, index := range idx {
		if index.Type != "" {
			ics[i].Type = hazelcastv1alpha1.EncodeIndexType[index.Type]
		}
		ics[i].Attributes = index.Attributes
		ics[i].Name = index.Name
		if index.BitmapIndexOptions != nil {
			ics[i].BitmapIndexOptions.UniqueKey = index.BitmapIndexOptions.UniqueKey
			if index.BitmapIndexOptions.UniqueKeyTransition != "" {
				ics[i].BitmapIndexOptions.UniqueKeyTransformation = hazelcastv1alpha1.EncodeUniqueKeyTransition[index.BitmapIndexOptions.UniqueKeyTransition]
			}
		}
	}

	return ics
}

func copyAttributes(attributes []hazelcastv1alpha1.AttributeConfig) []codecTypes.AttributeConfig {
	var atts []codecTypes.AttributeConfig

	for _, a := range attributes {
		atts = append(atts, codecTypes.AttributeConfig{
			Name:               a.Name,
			ExtractorClassName: a.ExtractorClassName,
		})
	}

	return atts
}

func (r *MapReconciler) updateLastSuccessfulConfiguration(ctx context.Context, m *hazelcastv1alpha1.Map) error {
	ms, err := json.Marshal(m.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.Update(ctx, r.Client, m, func() error {
		if m.ObjectMeta.Annotations == nil {
			m.ObjectMeta.Annotations = map[string]string{}
		}
		m.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(ms)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		r.Log.Info("Operation result", "Map Annotation", m.Name, "result", opResult)
	}
	return err
}

func (r *MapReconciler) validateMapConfigPersistence(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, m *hazelcastv1alpha1.Map) (bool, error) {
	cm := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: m.Spec.HazelcastResourceName, Namespace: m.Namespace}, cm)
	if err != nil {
		return false, fmt.Errorf("could not find Secret for map config persistence")
	}

	hzConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
	if err != nil {
		return false, fmt.Errorf("persisted Secret is not formatted correctly")
	}

	mcfg, ok := hzConfig.Hazelcast.Map[m.MapName()]
	if !ok {
		return false, nil
	}

	currentMcfg, err := createMapConfig(ctx, r.Client, h, m)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(mcfg, currentMcfg) { // TODO replace DeepEqual with custom implementation
		return false, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Map{}).
		Complete(r)
}
