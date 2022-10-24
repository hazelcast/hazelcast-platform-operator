package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
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
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
}

func NewMapReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}) *MapReconciler {
	return &MapReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
	}
}

const retryAfterForMap = 5 * time.Second

//+kubebuilder:rbac:groups=hazelcast.com,resources=maps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=maps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=maps/finalizers,verbs=update

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

	err = r.addFinalizer(ctx, m, logger)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
	}

	if m.GetDeletionTimestamp() != nil {
		updateMapStatus(ctx, r.Client, m, terminatingStatus(nil)) //nolint:errcheck
		err = r.executeFinalizer(ctx, m, logger)
		if err != nil {
			return updateMapStatus(ctx, r.Client, m, terminatingStatus(err).withMessage(err.Error()))
		}
		logger.V(2).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: m.Spec.HazelcastResourceName}, h)
	if err != nil {
		err = fmt.Errorf("could not create/update Map config: Hazelcast resource not found: %w", err)
		return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
	}
	if h.Status.Phase != hazelcastv1alpha1.Running {
		err = errors.NewServiceUnavailable("Hazelcast CR is not ready")
		return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
	}

	err = ValidatePersistence(m.Spec.PersistenceEnabled, h)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
	}

	s, createdBefore := m.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]

	if createdBefore {
		ms, err := json.Marshal(m.Spec)

		if err != nil {
			err = fmt.Errorf("error marshaling Map as JSON: %w", err)
			return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
		}
		if s == string(ms) {
			logger.Info("Map Config was already applied.", "name", m.Name, "namespace", m.Namespace)
			return updateMapStatus(ctx, r.Client, m, successStatus())
		}
		lastSpec := &hazelcastv1alpha1.MapSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last Map Spec: %w", err)
			return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
		}

		err = ValidateNotUpdatableFields(&m.Spec, lastSpec)
		if err != nil {
			return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
		}
	}

	cl, err := GetHazelcastClient(m)
	if err != nil {
		if errors.IsInternalError(err) {
			return updateMapStatus(ctx, r.Client, m, failedStatus(err).
				withMessage(err.Error()))
		}
		return updateMapStatus(ctx, r.Client, m, pendingStatus(retryAfterForMap).
			withMessage(err.Error()))
	}

	if m.Status.State != hazelcastv1alpha1.MapPersisting {
		requeue, err := updateMapStatus(ctx, r.Client, m, pendingStatus(0).withMessage("Applying new map configuration."))
		if err != nil {
			return requeue, err
		}
	}

	ms, err := r.ReconcileMapConfig(ctx, m, h, cl, createdBefore)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, pendingStatus(retryAfterForMap).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateMapStatus(ctx, r.Client, m, persistingStatus(1*time.Second).withMessage("Persisting the applied map config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateMapConfigPersistence(ctx, h, m)
	if err != nil {
		return updateMapStatus(ctx, r.Client, m, failedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateMapStatus(ctx, r.Client, m, persistingStatus(1*time.Second).withMessage("Waiting for Map Config to be persisted."))
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(m) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, m)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	return updateMapStatus(ctx, r.Client, m, successStatus().
		withMemberStatuses(nil))
}

func (r *MapReconciler) addFinalizer(ctx context.Context, m *hazelcastv1alpha1.Map, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(m, n.Finalizer) && m.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(m, n.Finalizer)
		err := r.Update(ctx, m)
		if err != nil {
			return err
		}
		logger.V(util.DebugLevel).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *MapReconciler) executeFinalizer(ctx context.Context, m *hazelcastv1alpha1.Map, logger logr.Logger) error {
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

func ValidatePersistence(pe bool, h *hazelcastv1alpha1.Hazelcast) error {
	if !pe {
		return nil
	}
	s, ok := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]

	if !ok {
		return fmt.Errorf("hazelcast resource %s is not successfully started yet", h.Name)
	}

	lastSpec := &hazelcastv1alpha1.HazelcastSpec{}
	err := json.Unmarshal([]byte(s), lastSpec)
	if err != nil {
		return fmt.Errorf("last successful spec for Hazelcast resource %s is not formatted correctly", h.Name)
	}

	if !lastSpec.Persistence.IsEnabled() {
		return fmt.Errorf("persistence is not enabled for the Hazelcast resource %s", h.Name)
	}

	return nil
}

func ValidateNotUpdatableFields(current *hazelcastv1alpha1.MapSpec, last *hazelcastv1alpha1.MapSpec) error {
	if current.Name != last.Name {
		return fmt.Errorf("name cannot be updated.")
	}
	if *current.BackupCount != *last.BackupCount {
		return fmt.Errorf("backupCount cannot be updated.")
	}
	if !util.IndexConfigSliceEquals(current.Indexes, last.Indexes) {
		return fmt.Errorf("indexes cannot be updated.")
	}
	if current.PersistenceEnabled != last.PersistenceEnabled {
		return fmt.Errorf("persistenceEnabled cannot be updated.")
	}
	if current.HazelcastResourceName != last.HazelcastResourceName {
		return fmt.Errorf("hazelcastResourceName cannot be updated")
	}
	return nil
}

func GetHazelcastClient(m *hazelcastv1alpha1.Map) (*hazelcast.Client, error) {
	hzcl, ok := hzclient.GetClient(types.NamespacedName{Name: m.Spec.HazelcastResourceName, Namespace: m.Namespace})
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("cannot connect to the cluster for %s", m.Spec.HazelcastResourceName))
	}
	if hzcl.Client == nil || !hzcl.Client.Running() {
		return nil, fmt.Errorf("trying to connect to the cluster %s", m.Spec.HazelcastResourceName)
	}

	return hzcl.Client, nil
}

func (r *MapReconciler) ReconcileMapConfig(
	ctx context.Context,
	m *hazelcastv1alpha1.Map,
	hz *hazelcastv1alpha1.Hazelcast,
	cl *hazelcast.Client,
	createdBefore bool,
) (map[string]hazelcastv1alpha1.MapConfigState, error) {
	ci := hazelcast.NewClientInternal(cl)
	var req *proto.ClientMessage
	if createdBefore {
		req = codec.EncodeMCUpdateMapConfigRequest(
			m.MapName(),
			*m.Spec.TimeToLiveSeconds,
			*m.Spec.MaxIdleSeconds,
			hazelcastv1alpha1.EncodeEvictionPolicyType[m.Spec.Eviction.EvictionPolicy],
			false,
			*m.Spec.Eviction.MaxSize,
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
	for _, member := range ci.OrderedMembers() {
		if status, ok := m.Status.MemberStatuses[member.UUID.String()]; ok && status == hazelcastv1alpha1.MapSuccess {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MapSuccess
			continue
		}
		_, err := ci.InvokeOnMember(ctx, req, member.UUID, nil)
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
	mapInput.TimeToLiveSeconds = *ms.TimeToLiveSeconds
	mapInput.MaxIdleSeconds = *ms.MaxIdleSeconds
	if ms.Eviction != nil {
		mapInput.EvictionConfig.EvictionPolicy = string(ms.Eviction.EvictionPolicy)
		mapInput.EvictionConfig.Size = *ms.Eviction.MaxSize
		mapInput.EvictionConfig.MaxSizePolicy = string(ms.Eviction.MaxSizePolicy)
	}
	mapInput.IndexConfigs = copyIndexes(ms.Indexes)
	mapInput.HotRestartConfig.Enabled = ms.PersistenceEnabled
	mapInput.WanReplicationRef = defaultWanReplicationRefCodec(hz, m)
	mapInput.InMemoryFormat = string(ms.InMemoryFormat)
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

func (r *MapReconciler) updateLastSuccessfulConfiguration(ctx context.Context, m *hazelcastv1alpha1.Map) error {
	ms, err := json.Marshal(m.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, m, func() error {
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
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: m.Spec.HazelcastResourceName, Namespace: m.Namespace}, cm)
	if err != nil {
		return false, fmt.Errorf("could not find ConfigMap for map config persistence")
	}

	hzConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal([]byte(cm.Data["hazelcast.yaml"]), hzConfig)
	if err != nil {
		return false, fmt.Errorf("persisted ConfigMap is not formatted correctly")
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
