package hazelcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/controllers"
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
	mtlsClientRegistry    mtls.HttpClientRegistry
	statusServiceRegistry hzclient.StatusServiceRegistry
}

func NewWanReplicationReconciler(client client.Client, log logr.Logger, scheme *runtime.Scheme, pht chan struct{}, mtlsClientRegistry mtls.HttpClientRegistry, cs hzclient.ClientRegistry, ssm hzclient.StatusServiceRegistry) *WanReplicationReconciler {
	return &WanReplicationReconciler{
		Client:                client,
		Logger:                log,
		Scheme:                scheme,
		phoneHomeTrigger:      pht,
		clientRegistry:        cs,
		mtlsClientRegistry:    mtlsClientRegistry,
		statusServiceRegistry: ssm,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/finalizers,verbs=update,namespace=watched

func (r *WanReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName, "seq", util.RandString(5))
	ctx = context.WithValue(ctx, LogKey("logger"), logger)

	wan := &hazelcastv1alpha1.WanReplication{}
	if err := r.Get(ctx, req.NamespacedName, wan); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(util.DebugLevel).Info("Could not find WanReplication, it is probably already deleted")
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	if !controllerutil.ContainsFinalizer(wan, n.Finalizer) && wan.GetDeletionTimestamp().IsZero() {
		controllerutil.AddFinalizer(wan, n.Finalizer)
		if err := r.Update(ctx, wan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !wan.GetDeletionTimestamp().IsZero() {
		updateWanStatus(ctx, r.Client, wan, recoptions.Empty(), withWanRepState(hazelcastv1alpha1.WanStatusTerminating)) //nolint:errcheck
		if controllerutil.ContainsFinalizer(wan, n.Finalizer) {
			if err := r.deleteLeftoverMapsFromStatus(ctx, wan); err != nil {
				return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err),
					withWanRepState(hazelcastv1alpha1.WanStatusTerminating),
					withWanRepMessage(err.Error()))
			}
			if err := r.stopWanReplicationAllMaps(ctx, wan); err != nil {
				return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err),
					withWanRepState(hazelcastv1alpha1.WanStatusTerminating),
					withWanRepMessage(err.Error()))
			}
			controllerutil.RemoveFinalizer(wan, n.Finalizer)
			if err := r.Update(ctx, wan); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if err := r.deleteLeftoverMapsFromStatus(ctx, wan); err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err),
			withWanRepFailedState(err.Error()))
	}

	hzClientMap, err := getMapsGroupByHazelcastName(ctx, r.Client, wan)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err),
			withWanRepFailedState(err.Error()))
	}

	if err := hazelcastv1alpha1.ValidateWanReplicationSpec(wan); err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	if err := r.checkWanEndpoint(ctx, wan); err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	err = r.stopWanRepForRemovedResources(ctx, wan, hzClientMap, r.clientRegistry)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	err = r.cleanupTerminatingMapCRs(ctx, wan, hzClientMap)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	err = r.checkConnectivity(ctx, req, wan, logger)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	if err := r.startWanReplication(ctx, wan, hzClientMap); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.addWanRepFinalizerToReplicatedMaps(ctx, wan, hzClientMap); err != nil {
		return ctrl.Result{}, err
	}

	persisted, err := r.validateWanConfigPersistence(ctx, wan, hzClientMap)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	if !persisted {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if util.IsPhoneHomeEnabled() && !recoptions.IsSuccessfullyApplied(wan) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, wan)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
		return updateWanStatus(ctx, r.Client, wan, recoptions.Error(err), withWanRepFailedState(err.Error()))
	}

	return ctrl.Result{}, nil
}

func (r *WanReplicationReconciler) deleteLeftoverMapsFromStatus(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) error {
	for mapWanKey, mapStatus := range wan.Status.WanReplicationMapsStatus {
		if mapStatus.PublisherId == "" {
			continue
		}

		if mapStatus.Status != hazelcastv1alpha1.WanStatusTerminating {
			continue
		}

		m, err := getWanMap(ctx, r.Client, types.NamespacedName{Name: mapStatus.ResourceName, Namespace: wan.Namespace})
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return err
			}
			// Map is not found, so somehow map deletion from status failed in previous reconcile triggers, try again
			if err := deleteWanMapStatus(ctx, r.Client, wan, mapWanKey); err != nil {
				return err
			}
		}
		if controllerutil.ContainsFinalizer(m, n.WanRepMapFinalizer) {
			// Finalizer will be removed in further steps in the reconciler
			return nil
		}
		// Finalizer was deleted from the map, we can safely remove map from WAN status.
		if err := deleteWanMapStatus(ctx, r.Client, wan, mapWanKey); err != nil {
			return err
		}
	}
	return nil
}

func (r *WanReplicationReconciler) stopWanReplicationAllMaps(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) error {
	publishedMaps, err := r.getPublishedMapsFromStatus(ctx, wan)
	if err != nil {
		return err
	}

	for mapWanKey := range wan.Status.WanReplicationMapsStatus {
		m, ok := publishedMaps[mapWanKey]
		if !ok {
			// Delete map without publisherID from status
			if err := deleteWanMapStatus(ctx, r.Client, wan, mapWanKey); err != nil {
				return err
			}
			continue
		}
		// Finalize and delete map with publisherID
		cli, err := getHazelcastClient(ctx, r.clientRegistry, m.Spec.HazelcastResourceName, m.Namespace)
		if err != nil {
			return err
		}
		if err := r.stopWanReplicationMap(ctx, wan, &m, cli); err != nil {
			return err
		}
	}

	if len := len(wan.Status.WanReplicationMapsStatus); len != 0 {
		return fmt.Errorf("Not all maps are finalized, number of left maps is %d", len)
	}
	return nil
}

func (r *WanReplicationReconciler) stopWanReplicationMap(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, m *hazelcastv1alpha1.Map, cli hzclient.Client) error {
	log := getLogger(ctx)
	mapWanKey := wanMapKey(m.Spec.HazelcastResourceName, m.MapName())

	// Check publisherId is registered to the status
	mapStatus, ok := wan.Status.WanReplicationMapsStatus[mapWanKey]
	if !ok {
		log.V(util.DebugLevel).Info("no key in the WAN status for", "mapKey", mapWanKey)
		return nil
	}

	publisherId := mapStatus.PublisherId
	if publisherId == "" {
		log.V(util.DebugLevel).Info("publisherId is empty, will remove map from WAN status", "mapKey", mapWanKey)
		if err := deleteWanMapStatus(ctx, r.Client, wan, mapWanKey); err != nil {
			return err
		}
		return nil
	}

	log.V(util.DebugLevel).Info("stopping WAN replication for", "mapKey", mapWanKey)
	ws := hzclient.NewWanService(cli, wanName(m.MapName()), publisherId)

	if err := ws.ChangeWanState(ctx, codecTypes.WanReplicationStateStopped); err != nil {
		return err
	}
	if err := ws.ClearWanQueue(ctx); err != nil {
		return err
	}
	if err := r.removeOwnerReference(ctx, wan, m, cli); err != nil {
		return err
	}
	// Finalizer removal from map and status deletion of map from WanReplication must be atomic
	// Marking Wan map status as terminating gives us chance to delete it from status if deletion in the following statements fails anyhow
	if err := updateWanMapStatus(ctx, r.Client, wan, mapWanKey, wanMapTerminatingStatus{}); err != nil {
		return err
	}
	if err := r.removeWanRepFinalizerFromMap(ctx, wan, m, cli); err != nil {
		return err
	}
	// Now map is safe to delete from WAN status. If following statement fails, status cleanup should be done in next trigger
	if err := deleteWanMapStatus(ctx, r.Client, wan, mapWanKey); err != nil {
		return err
	}
	return nil
}

func (r *WanReplicationReconciler) getPublishedMapsFromStatus(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) (map[string]hazelcastv1alpha1.Map, error) {
	hzMaps := make(map[string]hazelcastv1alpha1.Map, 0)
	for mapWanKey, status := range wan.Status.WanReplicationMapsStatus {
		if status.PublisherId == "" {
			continue
		}
		m, err := getWanMap(ctx, r.Client, types.NamespacedName{Name: status.ResourceName, Namespace: wan.Namespace})
		if err != nil {
			return nil, err
		}
		hzMaps[mapWanKey] = *m
	}

	return hzMaps, nil
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
					logger.V(util.DebugLevel).Info("Could not find", "Map", nn.Name)
				}
				return err
			}
			hzResourceName = m.Spec.HazelcastResourceName
		}

		name := types.NamespacedName{
			Namespace: req.Namespace,
			Name:      hzResourceName,
		}
		statusService, ok := r.statusServiceRegistry.Get(name)
		if !ok {
			return fmt.Errorf("get Hazelcast Status Service failed for Hazelcast CR %s", hzResourceName)
		}

		members := statusService.GetStatus().MemberDataMap
		var memberAddresses []string
		for _, v := range members {
			memberAddresses = append(memberAddresses, v.Address)
		}

		mtlsClient, ok := r.mtlsClientRegistry.Get(req.Namespace)
		if !ok {
			return errors.New("failed to get MTLS client")
		}
		for _, memberAddress := range memberAddresses {
			p, err := dialer.NewDialer(&dialer.Config{
				MemberAddress: memberAddress,
				MTLSClient:    mtlsClient,
			})
			if err != nil {
				return err
			}

			err = p.TryDial(ctx, splitEndpoints(wan.Spec.Endpoints))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func splitEndpoints(endpointsStr string) []string {
	endpoints := make([]string, 0)
	for _, endpoint := range strings.Split(endpointsStr, ",") {
		endpoint = strings.TrimSpace(endpoint)
		if len(endpoint) != 0 {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func joinEndpoints(endpoints []string) string {
	return strings.Join(endpoints, ",")
}

func (r *WanReplicationReconciler) removeWanRepFinalizerFromMap(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, m *hazelcastv1alpha1.Map, cli hzclient.Client) error {
	if !controllerutil.ContainsFinalizer(m, n.WanRepMapFinalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(m, n.WanRepMapFinalizer)
	if err := r.Update(ctx, m); err != nil {
		return err
	}
	return nil
}

func (r *WanReplicationReconciler) removeOwnerReference(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, m *hazelcastv1alpha1.Map, cli hzclient.Client) error {
	newOwnerRef := []metav1.OwnerReference{}
	for _, owner := range m.OwnerReferences {
		if owner.Kind == wan.Kind && owner.Name == wan.Name && owner.UID == wan.UID {
			continue
		}
		newOwnerRef = append(newOwnerRef, owner)
	}

	if len(newOwnerRef) == len(m.OwnerReferences) {
		return nil
	}

	m.OwnerReferences = newOwnerRef
	if err := r.Update(ctx, m); err != nil {
		return err
	}
	return nil
}

func (r *WanReplicationReconciler) stopWanRepForRemovedResources(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map, cs hzclient.ClientRegistry) error {
	mapsInSpec := make(map[string]hazelcastv1alpha1.Map)
	for hzName, maps := range hzClientMap {
		for _, m := range maps {
			mapsInSpec[wanMapKey(hzName, m.MapName())] = m
		}
	}

	wanMapStatus := wan.Status.WanReplicationMapsStatus
	for mapWanKey, status := range wanMapStatus {
		_, ok := mapsInSpec[mapWanKey]
		if ok {
			continue
		}
		// Map is deleted from spec, stop replication.
		m, err := getWanMap(ctx, r.Client, types.NamespacedName{Name: status.ResourceName, Namespace: wan.Namespace})
		if err != nil {
			return err
		}
		cli, err := getHazelcastClient(ctx, cs, m.Spec.HazelcastResourceName, m.Namespace)
		if err != nil {
			return err
		}
		if err := r.stopWanReplicationMap(ctx, wan, m, cli); err != nil {
			return err
		}
	}

	return nil
}

func (r *WanReplicationReconciler) cleanupTerminatingMapCRs(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map) error {
	for hzResourceName, maps := range hzClientMap {
		cli, err := getHazelcastClient(ctx, r.clientRegistry, hzResourceName, wan.Namespace)
		if err != nil {
			return err
		}

		for _, m := range maps {
			if m.Status.State != hazelcastv1alpha1.MapTerminating {
				continue
			}
			if err := r.stopWanReplicationMap(ctx, wan, &m, cli); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *WanReplicationReconciler) startWanReplication(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map) error {
	log := getLogger(ctx)

	mapWanStatus := make(map[string]WanMapStatusApplier)
	for hzResourceName, maps := range hzClientMap {
		cl, err := getHazelcastClient(ctx, r.clientRegistry, hzResourceName, wan.Namespace)
		if err != nil {
			return err
		}

		for _, m := range maps {
			mapWanKey := wanMapKey(hzResourceName, m.MapName())
			// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
			if wan.Status.WanReplicationMapsStatus[mapWanKey].PublisherId != "" {
				continue
			}

			if m.Status.State == hazelcastv1alpha1.MapTerminating {
				log.Info("Not applying WAN configuration to terminating map for ", "mapKey", mapWanKey)
				continue
			}

			if m.Status.State != hazelcastv1alpha1.MapSuccess {
				log.Info("Not applying WAN configuration for ", "mapKey", mapWanKey, "because the map state is ", m.Status.State)
				continue
			}

			log.Info("Applying WAN configuration for ", "mapKey", mapWanKey)
			publisherId, err := r.applyWanReplication(ctx, cl, wan, m.MapName(), mapWanKey)
			if err != nil {
				mapWanStatus[mapWanKey] = wanMapFailedStatus(err.Error())
				log.Info("Wan configuration failed for ", "mapKey", mapWanKey)
				continue
			}
			mapWanStatus[mapWanKey] = wanMapPersistingStatus{publisherId: publisherId, resourceName: m.Name}
			log.Info("Wan configuration successful for ", "mapKey", mapWanKey)
		}
	}

	if err := patchWanMapStatuses(ctx, r.Client, wan, mapWanStatus); err != nil {
		return err
	}
	return nil
}

func (r *WanReplicationReconciler) applyWanReplication(ctx context.Context, cli hzclient.Client, wan *hazelcastv1alpha1.WanReplication, mapName, mapWanKey string) (string, error) {
	publisherId := wan.Name + "-" + mapWanKey

	req := &hzclient.AddBatchPublisherRequest{
		TargetCluster:         wan.Spec.TargetClusterName,
		Endpoints:             wan.Spec.Endpoints,
		ResponseTimeoutMillis: wan.Spec.Acknowledgement.Timeout,
		AckType:               wan.Spec.Acknowledgement.Type,
		QueueCapacity:         wan.Spec.Queue.Capacity,
		QueueFullBehavior:     wan.Spec.Queue.FullBehavior,
		BatchSize:             wan.Spec.Batch.Size,
		BatchMaxDelayMillis:   wan.Spec.Batch.MaximumDelay,
	}

	ws := hzclient.NewWanService(cli, wanName(mapName), publisherId)
	err := ws.AddBatchPublisherConfig(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to apply WAN configuration: %w", err)
	}
	return publisherId, nil
}

func wanName(mapName string) string {
	return mapName + "-default"
}

func (r *WanReplicationReconciler) addWanRepFinalizerToReplicatedMaps(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map) error {
	for mapWanKey, status := range wan.Status.WanReplicationMapsStatus {
		if status.PublisherId == "" {
			continue
		}
		hzName, mapName := splitWanMapKey(mapWanKey)
		m, found := findMapInSlice(hzClientMap[hzName], mapName)
		if !found {
			return fmt.Errorf("Map %s is not present in the Hazelcast Client Map", mapName)
		}

		// Set owner reference for reconciler to get triggered when map is marked to be deleted
		// WAN Reconciler watches maps and filters map with owner references.
		if err := controllerutil.SetOwnerReference(wan, m, r.Scheme); err != nil {
			return err
		}

		if err := r.addWanRepFinalizerToMap(ctx, m); err != nil {
			return err
		}
	}

	return nil
}

func splitWanMapKey(key string) (hzName string, mapName string) {
	list := strings.Split(key, "__")
	return list[0], list[1]
}

func findMapInSlice(slice []hazelcastv1alpha1.Map, mapName string) (*hazelcastv1alpha1.Map, bool) {
	for i, mp := range slice {
		if mp.MapName() == mapName {
			return &slice[i], true
		}
	}
	return nil, false
}

func (r *WanReplicationReconciler) addWanRepFinalizerToMap(ctx context.Context, m *hazelcastv1alpha1.Map) error {
	if controllerutil.ContainsFinalizer(m, n.WanRepMapFinalizer) {
		return nil
	}

	controllerutil.AddFinalizer(m, n.WanRepMapFinalizer)

	return r.Update(ctx, m)
}

func (r *WanReplicationReconciler) validateWanConfigPersistence(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map) (bool, error) {
	cmMap := map[string]config.WanReplicationConfig{}

	// Fill map with WAN configs for each map wan key
	for hz := range hzClientMap {
		cm := &corev1.Secret{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: hz, Namespace: wan.Namespace}, cm)
		if err != nil {
			return false, fmt.Errorf("could not find Secret for wan config persistence")
		}

		hzConfig := &config.HazelcastWrapper{}
		err = yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
		if err != nil {
			return false, fmt.Errorf("persisted Secret is not formatted correctly")
		}

		// Add all map wan configs in Hazelcast Secret
		for wanName, wanConfig := range hzConfig.Hazelcast.WanReplication {
			mapName := splitWanName(wanName)
			cmMap[wanMapKey(hz, mapName)] = wanConfig
		}
	}

	mapWanStatus := make(map[string]WanMapStatusApplier)
	for mapWanKey, v := range wan.Status.WanReplicationMapsStatus {
		// Status is not equal to persisting, do nothing
		if v.Status != hazelcastv1alpha1.WanStatusPersisting {
			continue
		}

		// WAN is not in Config yet
		wanRep, ok := cmMap[mapWanKey]
		if !ok {
			continue
		}
		wanPub, ok := wanRep.BatchPublisher[v.PublisherId]
		if !ok {
			continue
		}

		// WAN publisher is in Config but is not correct
		realPub := createBatchPublisherConfig(*wan)
		if !reflect.DeepEqual(realPub, wanPub) {
			continue
		}
		mapWanStatus[mapWanKey] = wanMapSuccessStatus{}
	}

	if err := patchWanMapStatuses(ctx, r.Client, wan, mapWanStatus); err != nil {
		return false, err
	}

	if wan.Status.Status == hazelcastv1alpha1.WanStatusFailed {
		return false, fmt.Errorf("WAN replication for some maps failed")
	}

	if wan.Status.Status != hazelcastv1alpha1.WanStatusSuccess {
		return false, nil
	}

	return true, nil

}

func splitWanName(name string) string {
	return strings.TrimSuffix(name, "-default")
}

func (r *WanReplicationReconciler) updateLastSuccessfulConfiguration(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) error {
	ms, err := json.Marshal(wan.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.Update(ctx, r.Client, wan, func() error {
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

// mutating function
func (r *WanReplicationReconciler) checkWanEndpoint(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) error {
	endpoints := splitEndpoints(wan.Spec.Endpoints)
	for i, endpoint := range endpoints {
		_, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			endpoint = net.JoinHostPort(endpoint, strconv.Itoa(n.WanDefaultPort))
		}
		endpoints[i] = endpoint
	}
	endpointsStr := joinEndpoints(endpoints)
	if wan.Spec.Endpoints == endpointsStr {
		return nil
	}
	wan.Spec.Endpoints = endpointsStr
	return r.Client.Update(ctx, wan)
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
		Watches(&source.Kind{Type: &hazelcastv1alpha1.Map{}}, handler.EnqueueRequestsFromMapFunc(r.wanRequestsForSuccessfulMap),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(createEvent event.CreateEvent) bool {
					return false
				},
				UpdateFunc: func(updateEvent event.UpdateEvent) bool {
					// to run handler 'wanRequestsForSuccessfulMap' function after the map is ready
					oldMap, ok := updateEvent.ObjectOld.(*hazelcastv1alpha1.Map)
					if !ok {
						return false
					}
					newMap, ok := updateEvent.ObjectNew.(*hazelcastv1alpha1.Map)
					if !ok {
						return false
					}
					return oldMap.Status.State != hazelcastv1alpha1.MapSuccess && newMap.Status.State == hazelcastv1alpha1.MapSuccess
				},
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(genericEvent event.GenericEvent) bool {
					return false
				},
			}),
		).
		Watches(&source.Kind{Type: &hazelcastv1alpha1.Map{}}, handler.EnqueueRequestsFromMapFunc(r.wanRequestsForTerminationCandidateMap)).
		Complete(r)
}

func (r *WanReplicationReconciler) wanRequestsForSuccessfulMap(m client.Object) []reconcile.Request {
	hzMap, ok := m.(*hazelcastv1alpha1.Map)
	if !ok || hzMap.Status.State != hazelcastv1alpha1.MapSuccess {
		return []reconcile.Request{}
	}

	wanList := hazelcastv1alpha1.WanReplicationList{}
	nsMatcher := client.InNamespace(hzMap.Namespace)
	err := r.List(context.Background(), &wanList, nsMatcher)
	if err != nil {
		return []reconcile.Request{}
	}
	var requests []reconcile.Request
	for _, wan := range wanList.Items {
	wanResourcesLoop:
		for _, wanResource := range wan.Spec.Resources {
			switch wanResource.Kind {
			case hazelcastv1alpha1.ResourceKindMap:
				if wanResource.Name == hzMap.Name {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      wan.Name,
							Namespace: wan.Namespace,
						},
					})
					break wanResourcesLoop
				}
			case hazelcastv1alpha1.ResourceKindHZ:
				if wanResource.Name == hzMap.Spec.HazelcastResourceName {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      wan.Name,
							Namespace: wan.Namespace,
						},
					})
					break wanResourcesLoop
				}
			}
		}
	}
	return requests
}

func (r *WanReplicationReconciler) wanRequestsForTerminationCandidateMap(m client.Object) []reconcile.Request {
	mp, ok := m.(*hazelcastv1alpha1.Map)
	if !ok {
		return []reconcile.Request{}
	}

	if mp.Status.State != hazelcastv1alpha1.MapTerminating {
		return []reconcile.Request{}
	}

	// If map reconciler did not delete its finalizer yet
	for _, finalizer := range mp.Finalizers {
		if finalizer == n.Finalizer {
			return []reconcile.Request{}
		}
	}

	reqs := []reconcile.Request{}
	for _, owner := range mp.OwnerReferences {
		if owner.Kind == "WanReplication" {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      owner.Name,
					Namespace: mp.GetNamespace(),
				},
			})
		}
	}

	return reqs
}
