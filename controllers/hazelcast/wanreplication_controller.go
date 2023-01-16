package hazelcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"github.com/hazelcast/hazelcast-platform-operator/internal/dialer"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// WanReplicationReconciler reconciles a WanReplication object
type WanReplicationReconciler struct {
	client.Client
	logr.Logger
	Scheme                *runtime.Scheme
	phoneHomeTrigger      chan struct{}
	clientRegistry        hzclient.ClientRegistry
	mtlsClient            *mtls.Client
	statusServiceRegistry hzclient.StatusServiceRegistry
}

func NewWanReplicationReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme, pht chan struct{}, mtlsClient *mtls.Client, cs hzclient.ClientRegistry, ssm hzclient.StatusServiceRegistry) *WanReplicationReconciler {
	return &WanReplicationReconciler{
		Client:                client,
		Logger:                log,
		Scheme:                scheme,
		phoneHomeTrigger:      pht,
		clientRegistry:        cs,
		mtlsClient:            mtlsClient,
		statusServiceRegistry: ssm,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/finalizers,verbs=update

func (r *WanReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	wan := &hazelcastv1alpha1.WanReplication{}
	if err := r.Get(ctx, req.NamespacedName, wan); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(util.DebugLevel).Info("Could not find WanReplication, it is probably already deleted")
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	ctx = context.WithValue(ctx, LogKey("logger"), logger)

	if !controllerutil.ContainsFinalizer(wan, n.Finalizer) && wan.GetDeletionTimestamp().IsZero() {
		controllerutil.AddFinalizer(wan, n.Finalizer)
		logger.Info("Adding finalizer")
		if err := r.Update(ctx, wan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !wan.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(wan, n.Finalizer) {
			logger.Info("Deleting WAN configuration")
			if err := r.stopWanReplication(ctx, wan); err != nil {
				return updateWanStatus(ctx, r.Client, wan, wanTerminatingStatus().withMessage(err.Error()))
			}
			logger.Info("Deleting WAN configuration finalizer")
			controllerutil.RemoveFinalizer(wan, n.Finalizer)
			if err := r.Update(ctx, wan); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	err := r.checkConnectivity(ctx, req, wan, logger)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	if !util.IsApplied(wan) {
		if err := r.Update(ctx, insertLastAppliedSpec(wan)); err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
		} else {
			return updateWanStatus(ctx, r.Client, wan, wanPendingStatus())
		}
	}

	HZClientMap, err := r.getMapsGroupByHazelcastName(ctx, wan)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	s, createdBefore := wan.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if createdBefore {
		ms, err := json.Marshal(wan.Spec)

		if err != nil {
			err = fmt.Errorf("error marshaling WanReplication as JSON: %w", err)
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
		}

		if s == string(ms) {
			logger.Info("WanReplication Config was already applied.", "name", wan.Name, "namespace", wan.Namespace)
			return updateWanStatus(ctx, r.Client, wan, wanSuccessStatus())
		}

		lastSpec := &hazelcastv1alpha1.WanReplicationSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last WanReplication Spec: %w", err)
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
		}

		err = validateNotUpdatableFields(&wan.Spec, lastSpec)
		if err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
		}

		err = stopWanRepForRemovedResources(ctx, wan, HZClientMap, r.clientRegistry)
		if err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
		}

	}

	err = r.startWanReplication(ctx, wan, HZClientMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	requeue, err := updateWanStatus(ctx, r.Client, wan, wanPersistingStatus(retryAfter).withMessage("Persisting the applied map config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateWanConfigPersistence(ctx, wan, HZClientMap)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateWanStatus(ctx, r.Client, wan, wanPersistingStatus(retryAfter).withMessage("Waiting for Wan Config to be persisted."))
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(wan) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, wan)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	return updateWanStatus(ctx, r.Client, wan, wanSuccessStatus())
}

func (r *WanReplicationReconciler) checkConnectivity(ctx context.Context, req ctrl.Request, wan *hazelcastv1alpha1.WanReplication, logger logr.Logger) error {
	for _, rr := range wan.Spec.Resources {
		hzResourceName := rr.Name
		if rr.Kind == hazelcastv1alpha1.ResourceKindMap {
			m := &hazelcastv1alpha1.Map{}
			nn := types.NamespacedName{
				Namespace: req.NamespacedName.Namespace,
				Name:      rr.Name,
			}
			if err := r.Get(ctx, nn, m); err != nil {
				if kerrors.IsNotFound(err) {
					logger.V(util.DebugLevel).Info("Could not find Map")
				}
				return err
			}
			hzResourceName = m.Spec.HazelcastResourceName
		}

		statusService, ok := r.statusServiceRegistry.Get(types.NamespacedName{
			Namespace: req.Namespace,
			Name:      hzResourceName,
		})
		if !ok {
			return errors.New("get Hazelcast Status Service failed")
		}

		members := statusService.GetStatus().MemberDataMap
		var memberAddresses []string
		for _, v := range members {
			memberAddresses = append(memberAddresses, v.Address)
		}

		for _, memberAddress := range memberAddresses {
			p, err := dialer.NewDialer(&dialer.Config{
				MemberAddress: memberAddress,
				MTLSClient:    r.mtlsClient,
			})
			if err != nil {
				return err
			}

			err = p.TryDial(ctx, wan.Spec.Endpoints)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *WanReplicationReconciler) startWanReplication(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, HZClientMap map[string][]hazelcastv1alpha1.Map) error {
	log := getLogger(ctx)

	mapWanStatus := make(map[string]wanOptionsBuilder)
	for hzResourceName, maps := range HZClientMap {
		cl, err := GetHazelcastClient(r.clientRegistry, &maps[0])
		if err != nil {
			return err
		}

		for _, m := range maps {
			mapWanKey := wanMapKey(hzResourceName, m.MapName())
			// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
			if wan.Status.WanReplicationMapsStatus[mapWanKey].PublisherId == "" {
				log.Info("Applying WAN configuration for ", "mapKey", mapWanKey)
				if publisherId, err := r.applyWanReplication(ctx, cl, wan, m.MapName(), mapWanKey); err != nil {
					mapWanStatus[mapWanKey] = wanFailedStatus(err).withMessage(err.Error())
				} else {
					mapWanStatus[mapWanKey] = wanPersistingStatus(0).withPublisherId(publisherId)
				}

			}
		}
	}

	if err := putWanMapStatus(ctx, r.Client, wan, mapWanStatus); err != nil {
		return err
	}
	return nil
}

func (r *WanReplicationReconciler) validateWanConfigPersistence(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, HZClientMap map[string][]hazelcastv1alpha1.Map) (bool, error) {
	cmMap := map[string]config.WanReplicationConfig{}

	// Fill map with Wan configs for each map wan key
	for hz := range HZClientMap {
		cm := &corev1.ConfigMap{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: hz, Namespace: wan.Namespace}, cm)
		if err != nil {
			return false, fmt.Errorf("could not find ConfigMap for wan config persistence")
		}

		hzConfig := &config.HazelcastWrapper{}
		err = yaml.Unmarshal([]byte(cm.Data["hazelcast.yaml"]), hzConfig)
		if err != nil {
			return false, fmt.Errorf("persisted ConfigMap is not formatted correctly")
		}

		// Add all map wan configs in Hazelcast ConfigMap
		for wanName, wanConfig := range hzConfig.Hazelcast.WanReplication {
			mapName := splitWanName(wanName)
			cmMap[wanMapKey(hz, mapName)] = wanConfig
		}
	}

	mapWanStatus := make(map[string]wanOptionsBuilder)
	for mapWanKey, v := range wan.Status.WanReplicationMapsStatus {
		// Status is not equal to persisting, do nothing
		if v.Status != hazelcastv1alpha1.WanStatusPersisting {
			continue
		}

		// Wan is not in ConfigMap yet
		wanRep, ok := cmMap[mapWanKey]
		if !ok {
			continue
		}

		// Wan is in ConfigMap but is not correct
		realWan := createWanReplicationConfig(v.PublisherId, *wan)
		if !reflect.DeepEqual(realWan, wanRep) {
			continue
		}

		mapWanStatus[mapWanKey] = wanSuccessStatus().withPublisherId(v.PublisherId)
	}

	if err := putWanMapStatus(ctx, r.Client, wan, mapWanStatus); err != nil {
		return false, err
	}

	if wan.Status.Status == hazelcastv1alpha1.WanStatusFailed {
		return false, fmt.Errorf("Wan replication for some maps failed")
	}

	if wan.Status.Status != hazelcastv1alpha1.WanStatusSuccess {
		return false, nil
	}

	return true, nil

}

func (r *WanReplicationReconciler) getMapsGroupByHazelcastName(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) (map[string][]hazelcastv1alpha1.Map, error) {
	HZClientMap := make(map[string][]hazelcastv1alpha1.Map)
	for _, resource := range wan.Spec.Resources {
		switch resource.Kind {
		case hazelcastv1alpha1.ResourceKindMap:
			m, err := r.getWanMap(ctx, types.NamespacedName{Name: resource.Name, Namespace: wan.Namespace}, true)
			if err != nil {
				return nil, err
			}
			mapList, ok := HZClientMap[m.Spec.HazelcastResourceName]
			if !ok {
				HZClientMap[m.Spec.HazelcastResourceName] = []hazelcastv1alpha1.Map{*m}
			}
			HZClientMap[m.Spec.HazelcastResourceName] = append(mapList, *m)
		case hazelcastv1alpha1.ResourceKindHZ:
			maps, err := r.getAllMapsInHazelcast(ctx, resource.Name, wan.Namespace)
			if err != nil {
				return nil, err
			}
			// If no map is present for the Hazelcast resource
			if len(maps) == 0 {
				continue
			}
			mapList, ok := HZClientMap[resource.Name]
			if !ok {
				HZClientMap[resource.Name] = maps
			}
			HZClientMap[resource.Name] = append(mapList, maps...)
		}
	}
	return HZClientMap, nil
}

func (r *WanReplicationReconciler) getAllMapsInHazelcast(ctx context.Context, hazelcastResourceName string, wanNamespace string) ([]hazelcastv1alpha1.Map, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": hazelcastResourceName}
	nsMatcher := client.InNamespace(wanNamespace)

	wrl := &hazelcastv1alpha1.MapList{}

	if err := r.Client.List(ctx, wrl, fieldMatcher, nsMatcher); err != nil {
		return nil, fmt.Errorf("could not get Map resources dependent under given Hazelcast %w", err)
	}
	return wrl.Items, nil
}

func validateNotUpdatableFields(current *hazelcastv1alpha1.WanReplicationSpec, last *hazelcastv1alpha1.WanReplicationSpec) error {
	if current.TargetClusterName != last.TargetClusterName {
		return fmt.Errorf("targetClusterName cannot be updated")
	}
	if current.Endpoints != last.Endpoints {
		return fmt.Errorf("endpoints cannot be updated")
	}
	if current.Queue != last.Queue {
		return fmt.Errorf("queue cannot be updated")
	}
	if current.Batch != last.Batch {
		return fmt.Errorf("batch cannot be updated")
	}
	if current.Acknowledgement != last.Acknowledgement {
		return fmt.Errorf("acknowledgement cannot be updated")
	}
	return nil
}

func (r *WanReplicationReconciler) getWanMap(ctx context.Context, lk types.NamespacedName, checkSuccess bool) (*hazelcastv1alpha1.Map, error) {
	m := &hazelcastv1alpha1.Map{}
	if err := r.Client.Get(ctx, lk, m); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get Map CR from WanReplication: %w", err)
	}

	if checkSuccess && m.Status.State != hazelcastv1alpha1.MapSuccess {
		return nil, fmt.Errorf("status of map %s is not success", m.Name)
	}

	return m, nil

}

func (r *WanReplicationReconciler) applyWanReplication(ctx context.Context, cli hzclient.Client, wan *hazelcastv1alpha1.WanReplication, mapName, mapWanKey string) (string, error) {
	publisherId := wan.Name + "-" + mapWanKey

	req := &hzclient.AddBatchPublisherRequest{
		TargetCluster:         wan.Spec.TargetClusterName,
		Endpoints:             wan.Spec.Endpoints,
		QueueCapacity:         wan.Spec.Queue.Capacity,
		BatchSize:             wan.Spec.Batch.Size,
		BatchMaxDelayMillis:   wan.Spec.Batch.MaximumDelay,
		ResponseTimeoutMillis: wan.Spec.Acknowledgement.Timeout,
		AckType:               wan.Spec.Acknowledgement.Type,
		QueueFullBehavior:     wan.Spec.Queue.FullBehavior,
	}

	ws := hzclient.NewWanService(cli, wanName(mapName), publisherId)
	err := ws.AddBatchPublisherConfig(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to apply WAN configuration: %w", err)
	}
	return publisherId, nil
}

func (r *WanReplicationReconciler) stopWanReplication(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) error {
	HZClientMap, err := r.getMapsGroupByHazelcastName(ctx, wan)
	if err != nil {
		return err
	}

	log := getLogger(ctx)

	for hzResourceName, maps := range HZClientMap {

		cli, err := GetHazelcastClient(r.clientRegistry, &maps[0])
		if err != nil {
			return err
		}

		for _, m := range maps {
			mapWanKey := wanMapKey(hzResourceName, m.MapName())
			// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
			publisherId := wan.Status.WanReplicationMapsStatus[mapWanKey].PublisherId
			if publisherId == "" {
				log.V(util.DebugLevel).Info("publisherId is empty, will skip stopping WAN replication", "mapKey", mapWanKey)
				continue
			}
			ws := hzclient.NewWanService(cli, wanName(m.MapName()), publisherId)

			if err := ws.ChangeWanState(ctx, codecTypes.WanReplicationStateStopped); err != nil {
				return err
			}
			if err := ws.ClearWanQueue(ctx); err != nil {
				return err
			}
			delete(wan.Status.WanReplicationMapsStatus, mapWanKey)
		}
	}
	return nil
}

func stopWanRepForRemovedResources(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, HZClientMap map[string][]hazelcastv1alpha1.Map, cs hzclient.ClientRegistry) error {
	tempMapSet := make(map[string]hazelcastv1alpha1.Map)
	for hzName, maps := range HZClientMap {
		for _, m := range maps {
			tempMapSet[wanMapKey(hzName, m.MapName())] = m
		}
	}

	for mapWanKey, status := range wan.Status.WanReplicationMapsStatus {
		m, ok := tempMapSet[mapWanKey]
		if ok {
			continue
		}
		cli, err := GetHazelcastClient(cs, &m)
		if err != nil {
			return err
		}
		ws := hzclient.NewWanService(cli, wanName(m.MapName()), status.PublisherId)
		if err := ws.ChangeWanState(ctx, codecTypes.WanReplicationStateStopped); err != nil {
			return err
		}
		if err := ws.ClearWanQueue(ctx); err != nil {
			return err
		}
		delete(wan.Status.WanReplicationMapsStatus, mapWanKey)
	}
	return nil
}

func wanMapKey(hzName, mapName string) string {
	return hzName + "__" + mapName
}

func splitWanMapKey(key string) (hzName string, mapName string) {
	list := strings.Split(key, "__")
	return list[0], list[1]
}

func wanName(mapName string) string {
	return mapName + "-default"
}

func splitWanName(name string) string {
	return strings.TrimSuffix(name, "-default")
}

func insertLastAppliedSpec(wan *hazelcastv1alpha1.WanReplication) *hazelcastv1alpha1.WanReplication {
	b, _ := json.Marshal(wan.Spec)
	if wan.Annotations == nil {
		wan.Annotations = make(map[string]string)
	}
	wan.Annotations[n.LastAppliedSpecAnnotation] = string(b)
	return wan
}

func (r *WanReplicationReconciler) updateLastSuccessfulConfiguration(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) error {
	ms, err := json.Marshal(wan.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, wan, func() error {
		if wan.ObjectMeta.Annotations == nil {
			wan.ObjectMeta.Annotations = map[string]string{}
		}

		wan.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(ms)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		r.Logger.Info("Operation result", "WanReplication Annotation", wan.Name, "result", opResult)
	}
	return err
}

type LogKey string

var ctxLogger = LogKey("logger")

func getLogger(ctx context.Context) logr.Logger {
	return ctx.Value(ctxLogger).(logr.Logger)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.WanReplication{}).
		Complete(r)
}
