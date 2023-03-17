package hazelcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
		if err := r.Update(ctx, wan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !wan.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(wan, n.Finalizer) {
			if err := r.deleteLeftoverMapsFromStatus(ctx, wan); err != nil {
				return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
			}
			if err := r.stopWanReplicationAllMaps(ctx, wan); err != nil {
				return updateWanStatus(ctx, r.Client, wan, wanTerminatingStatus().withMessage(err.Error()))
			}
			controllerutil.RemoveFinalizer(wan, n.Finalizer)
			if err := r.Update(ctx, wan); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if err := r.deleteLeftoverMapsFromStatus(ctx, wan); err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	hzClientMap, err := r.getMapsGroupByHazelcastName(ctx, wan)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	if s, successfullyAppliedBefore := wan.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]; successfullyAppliedBefore {
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
	}

	err = r.stopWanRepForRemovedResources(ctx, wan, hzClientMap, r.clientRegistry)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	err = r.cleanupTerminatingMapCRs(ctx, wan, hzClientMap)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	err = r.checkConnectivity(ctx, req, wan, logger)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	if err := r.startWanReplication(ctx, wan, hzClientMap); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.addWanRepFinalizerToReplicatedMaps(ctx, wan, hzClientMap); err != nil {
		return ctrl.Result{}, err
	}

	persisted, err := r.validateWanConfigPersistence(ctx, wan, hzClientMap)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(wan) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, wan)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus(err).withMessage(err.Error()))
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

		m, err := r.getWanMap(ctx, types.NamespacedName{Name: mapStatus.ResourceName, Namespace: wan.Namespace})
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
	// Marking WAN map status as terminating gives us chance to delete it from status if deletion in the following statements fails anyhow
	if err := updateWanMapStatus(ctx, r.Client, wan, mapWanKey, hazelcastv1alpha1.WanStatusTerminating); err != nil {
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
		m, err := r.getWanMap(ctx, types.NamespacedName{Name: status.ResourceName, Namespace: wan.Namespace})
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

		statusService, ok := r.statusServiceRegistry.Get(types.NamespacedName{
			Namespace: req.Namespace,
			Name:      hzResourceName,
		})
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

			err = p.TryDial(ctx, wan.Spec.Endpoints)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *WanReplicationReconciler) getMapsGroupByHazelcastName(ctx context.Context, wan *hazelcastv1alpha1.WanReplication) (map[string][]hazelcastv1alpha1.Map, error) {
	hzClientMap := make(map[string][]hazelcastv1alpha1.Map)
	for _, resource := range wan.Spec.Resources {
		switch resource.Kind {
		case hazelcastv1alpha1.ResourceKindMap:
			m, err := r.getWanMap(ctx, types.NamespacedName{Name: resource.Name, Namespace: wan.Namespace})
			if err != nil {
				return nil, err
			}
			mapList, ok := hzClientMap[m.Spec.HazelcastResourceName]
			if !ok {
				hzClientMap[m.Spec.HazelcastResourceName] = []hazelcastv1alpha1.Map{*m}
			}
			hzClientMap[m.Spec.HazelcastResourceName] = append(mapList, *m)
		case hazelcastv1alpha1.ResourceKindHZ:
			maps, err := r.getAllMapsInHazelcast(ctx, resource.Name, wan.Namespace)
			if err != nil {
				return nil, err
			}
			// If no map is present for the Hazelcast resource
			if len(maps) == 0 {
				continue
			}
			mapList, ok := hzClientMap[resource.Name]
			if !ok {
				hzClientMap[resource.Name] = maps
			}
			hzClientMap[resource.Name] = append(mapList, maps...)
		}
	}
	for k, v := range hzClientMap {
		hzClientMap[k] = removeDuplicate(v)
	}

	return hzClientMap, nil
}

func (r *WanReplicationReconciler) getWanMap(ctx context.Context, lk types.NamespacedName) (*hazelcastv1alpha1.Map, error) {
	m := &hazelcastv1alpha1.Map{}
	if err := r.Client.Get(ctx, lk, m); err != nil {
		return nil, fmt.Errorf("failed to get Map CR from WanReplication: %w", err)
	}

	return m, nil
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

func removeDuplicate(mapList []hazelcastv1alpha1.Map) []hazelcastv1alpha1.Map {
	keySet := make(map[types.NamespacedName]struct{})
	list := []hazelcastv1alpha1.Map{}
	for _, item := range mapList {
		nsname := types.NamespacedName{Name: item.Name, Namespace: item.Namespace}
		if _, ok := keySet[nsname]; !ok {
			keySet[nsname] = struct{}{}
			list = append(list, item)
		}
	}
	return list
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
		m, err := r.getWanMap(ctx, types.NamespacedName{Name: status.ResourceName, Namespace: wan.Namespace})
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

	mapWanStatus := make(map[string]wanOptionsBuilder)
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
				mapWanStatus[mapWanKey] = wanFailedStatus(err).withMessage(err.Error())
				log.Info("WAN configuration failed for ", "mapKey", mapWanKey)
				continue
			}
			mapWanStatus[mapWanKey] = wanPersistingStatus(0).withPublisherId(publisherId).withResourceName(m.Name)
			log.Info("WAN configuration successful for ", "mapKey", mapWanKey)
		}
	}

	if err := putWanMapStatus(ctx, r.Client, wan, mapWanStatus); err != nil {
		return err
	}
	return nil
}

func wanMapKey(hzName, mapName string) string {
	return hzName + "__" + mapName
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
	if err := r.Update(ctx, m); err != nil {
		return err
	}

	return nil
}

func (r *WanReplicationReconciler) validateWanConfigPersistence(ctx context.Context, wan *hazelcastv1alpha1.WanReplication, hzClientMap map[string][]hazelcastv1alpha1.Map) (bool, error) {
	cmMap := map[string]config.WanReplicationConfig{}

	// Fill map with WAN configs for each map wan key
	for hz := range hzClientMap {
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

		// WAN is not in ConfigMap yet
		wanRep, ok := cmMap[mapWanKey]
		if !ok {
			continue
		}

		// WAN is in ConfigMap but is not correct
		realWan := createWanReplicationConfig(v.PublisherId, *wan)
		if !reflect.DeepEqual(realWan, wanRep) {
			continue
		}

		mapWanStatus[mapWanKey] = wanSuccessStatus().withPublisherId(v.PublisherId).withResourceName(v.ResourceName)
	}

	if err := putWanMapStatus(ctx, r.Client, wan, mapWanStatus); err != nil {
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
		Watches(&source.Kind{Type: &hazelcastv1alpha1.Map{}}, handler.EnqueueRequestsFromMapFunc(r.terminatingMapUpdates)).
		Complete(r)
}

func (r *WanReplicationReconciler) terminatingMapUpdates(m client.Object) []reconcile.Request {
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
