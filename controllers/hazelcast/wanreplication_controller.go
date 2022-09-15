package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// WanReplicationReconciler reconciles a WanReplication object
type WanReplicationReconciler struct {
	client.Client
	logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
}

func NewWanReplicationReconciler(
	client client.Client, log logr.Logger, scheme *runtime.Scheme, pht chan struct{}) *WanReplicationReconciler {
	return &WanReplicationReconciler{
		Client:           client,
		Logger:           log,
		Scheme:           scheme,
		phoneHomeTrigger: pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/finalizers,verbs=update

func (r *WanReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	wan := &hazelcastcomv1alpha1.WanReplication{}
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
				return ctrl.Result{}, err
			}
			logger.Info("Deleting WAN configuration finalizer")
			controllerutil.RemoveFinalizer(wan, n.Finalizer)
			if err := r.Update(ctx, wan); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	HZClientMap, err := r.getMapsGroupByHazelcastName(ctx, wan)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
	}

	s, createdBefore := wan.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if createdBefore {
		ms, err := json.Marshal(wan.Spec)

		if err != nil {
			err = fmt.Errorf("error marshaling WanReplication as JSON: %w", err)
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		}

		if s == string(ms) {
			logger.Info("WanReplication Config was already applied.", "name", wan.Name, "namespace", wan.Namespace)
			return updateWanStatus(ctx, r.Client, wan, wanSuccessStatus())
		}

		lastSpec := &hazelcastcomv1alpha1.WanReplicationSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last WanReplication Spec: %w", err)
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		}

		err = validateNotUpdatableFields(&wan.Spec, lastSpec)
		if err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		}

		err = stopWanRepForRemovedResources(ctx, wan, HZClientMap)
		if err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		}

	}

	err = r.startWanReplication(ctx, wan, HZClientMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !util.IsSuccessfullyApplied(wan) {
		if util.IsPhoneHomeEnabled() {
			go func() { r.phoneHomeTrigger <- struct{}{} }()
		}
		if err := r.Update(ctx, insertLastSuccessfullyAppliedSpec(wan)); err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		}
	}

	if !isWanSuccessful(wan) {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage("WAN replication is not successfully applied to some maps"))
	}

	return updateWanStatus(ctx, r.Client, wan, wanSuccessStatus())
}

func (r *WanReplicationReconciler) startWanReplication(ctx context.Context, wan *hazelcastcomv1alpha1.WanReplication, HZClientMap map[string][]hazelcastcomv1alpha1.Map) error {
	log := getLogger(ctx)

	mapWanStatus := make(map[string]wanOptionsBuilder)
	for hzResourceName, maps := range HZClientMap {
		cli, err := GetHazelcastClient(&maps[0])
		if err != nil {
			return err
		}

		for _, m := range maps {
			mapWanKey := mapWanReplicationKey(hzResourceName, m.MapName())
			// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
			if wan.Status.WanReplicationMapsStatus[mapWanKey].PublisherId == "" {
				log.Info("Applying WAN configuration for ", "mapKey", mapWanKey)
				// TODO: ayni publisherID apply edildiğinde hata donuyor mu donuyorsa ne hatasi cek et.
				if publisherId, err := r.applyWanReplication(ctx, cli, wan, m.MapName(), mapWanKey); err != nil {
					mapWanStatus[mapWanKey] = wanFailedStatus().withMessage(err.Error())
				} else {
					mapWanStatus[mapWanKey] = wanSuccessStatus().withPublisherId(publisherId)
				}

			}
		}
	}

	if err := putWanMapStatus(ctx, r.Client, wan, mapWanStatus); err != nil {
		return err
	}
	return nil
}

func (r *WanReplicationReconciler) getMapsGroupByHazelcastName(ctx context.Context, wan *hazelcastcomv1alpha1.WanReplication) (map[string][]hazelcastcomv1alpha1.Map, error) {
	HZClientMap := make(map[string][]hazelcastcomv1alpha1.Map)
	for _, resource := range wan.Spec.Resources {
		switch resource.Type {
		case hazelcastcomv1alpha1.ResourceTypeMap:
			m, err := r.getWanMap(ctx, types.NamespacedName{Name: resource.Name, Namespace: wan.Namespace}, true)
			if err != nil {
				return nil, err
			}
			mapList, ok := HZClientMap[m.Spec.HazelcastResourceName]
			if !ok {
				HZClientMap[m.Spec.HazelcastResourceName] = []hazelcastcomv1alpha1.Map{*m}
			}
			HZClientMap[m.Spec.HazelcastResourceName] = append(mapList, *m)
		case hazelcastcomv1alpha1.ResourceTypeHZ:
			maps, err := r.getAllMapsInHazelcast(ctx, resource.Name, wan.Namespace)
			if err != nil {
				return nil, err
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

func (r *WanReplicationReconciler) getAllMapsInHazelcast(ctx context.Context, hazelcastResourceName string, wanNamespace string) ([]hazelcastcomv1alpha1.Map, error) {
	fieldMatcher := client.MatchingFields{"HazelcastResourceName": hazelcastResourceName}
	nsMatcher := client.InNamespace(wanNamespace)

	wrl := &hazelcastcomv1alpha1.MapList{}

	if err := r.Client.List(ctx, wrl, fieldMatcher, nsMatcher); err != nil {
		return nil, fmt.Errorf("could not get Map resources dependent under given Hazelcast %w", err)
	}
	return wrl.Items, nil
}

func validateNotUpdatableFields(current *hazelcastcomv1alpha1.WanReplicationSpec, last *hazelcastcomv1alpha1.WanReplicationSpec) error {
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

func (r *WanReplicationReconciler) getWanMap(ctx context.Context, lk types.NamespacedName, checkSuccess bool) (*hazelcastcomv1alpha1.Map, error) {
	m := &hazelcastcomv1alpha1.Map{}
	if err := r.Client.Get(ctx, lk, m); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get Map CR from WanReplication: %w", err)
	}

	if checkSuccess && m.Status.State != hazelcastcomv1alpha1.MapSuccess {
		return nil, fmt.Errorf("status of map %s is not success", m.Name)
	}

	return m, nil

}

func (r *WanReplicationReconciler) applyWanReplication(ctx context.Context, client *hazelcast.Client, wan *hazelcastcomv1alpha1.WanReplication, mapName, mapWanKey string) (string, error) {
	publisherId := wan.Name + "-" + mapWanKey

	req := &addBatchPublisherRequest{
		hazelcastWanReplicationName(mapName),
		wan.Spec.TargetClusterName,
		publisherId,
		wan.Spec.Endpoints,
		wan.Spec.Queue.Capacity,
		wan.Spec.Batch.Size,
		wan.Spec.Batch.MaximumDelay,
		wan.Spec.Acknowledgement.Timeout,
		convertAckType(wan.Spec.Acknowledgement.Type),
		convertQueueBehavior(wan.Spec.Queue.FullBehavior),
	}

	err := addBatchPublisherConfig(ctx, client, req)
	if err != nil {
		return "", fmt.Errorf("failed to apply WAN configuration: %w", err)
	}
	return publisherId, nil
}

func (r *WanReplicationReconciler) stopWanReplication(ctx context.Context, wan *hazelcastcomv1alpha1.WanReplication) error {
	HZClientMap, err := r.getMapsGroupByHazelcastName(ctx, wan)
	if err != nil {
		return err
	}

	log := getLogger(ctx)

	for hzResourceName, maps := range HZClientMap {

		cli, err := GetHazelcastClient(&maps[0])
		if err != nil {
			return err
		}

		for _, m := range maps {
			mapWanKey := mapWanReplicationKey(hzResourceName, m.MapName())
			// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
			publisherId := wan.Status.WanReplicationMapsStatus[mapWanKey].PublisherId
			if publisherId == "" {
				log.V(util.DebugLevel).Info("publisherId is empty, will skip stopping WAN replication", "mapKey", mapWanKey)
				continue
			}
			req := &changeWanStateRequest{
				name:        hazelcastWanReplicationName(m.MapName()),
				publisherId: publisherId,
				state:       codecTypes.WanReplicationStateStopped,
			}

			if err := changeWanState(ctx, cli, req); err != nil {
				return err
			}
		}
	}
	return nil
}

func stopWanRepForRemovedResources(ctx context.Context, wan *hazelcastcomv1alpha1.WanReplication, HZClientMap map[string][]hazelcastcomv1alpha1.Map) error {

	tempMapSet := make(map[string]hazelcastcomv1alpha1.Map)
	for hzName, maps := range HZClientMap {
		for _, m := range maps {
			tempMapSet[mapWanReplicationKey(hzName, m.MapName())] = m
		}
	}

	for mapWanKey, status := range wan.Status.WanReplicationMapsStatus {
		m, ok := tempMapSet[mapWanKey]
		if ok {
			continue
		}
		req := &changeWanStateRequest{
			name:        hazelcastWanReplicationName(m.MapName()),
			publisherId: status.PublisherId,
			state:       codecTypes.WanReplicationStateStopped,
		}

		cli, err := GetHazelcastClient(&m)
		if err != nil {
			return err
		}
		if err = changeWanState(ctx, cli, req); err != nil {
			return err
		}
		delete(wan.Status.WanReplicationMapsStatus, mapWanKey)
	}
	return nil
}

func mapWanReplicationKey(hzName, mapName string) string {
	return hzName + "_" + mapName
}

func hazelcastWanReplicationName(mapName string) string {
	return mapName + "-default"
}

type addBatchPublisherRequest struct {
	name                  string
	targetCluster         string
	publisherId           string
	endpoints             string
	queueCapacity         int32
	batchSize             int32
	batchMaxDelayMillis   int32
	responseTimeoutMillis int32
	ackType               int32
	queueFullBehavior     int32
}

func addBatchPublisherConfig(
	ctx context.Context,
	client *hazelcast.Client,
	request *addBatchPublisherRequest,
) error {
	cliInt := hazelcast.NewClientInternal(client)

	req := codec.EncodeMCAddWanBatchPublisherConfigRequest(
		request.name,
		request.targetCluster,
		request.publisherId,
		request.endpoints,
		request.queueCapacity,
		request.batchSize,
		request.batchMaxDelayMillis,
		request.responseTimeoutMillis,
		request.ackType,
		request.queueFullBehavior,
	)

	for _, member := range cliInt.OrderedMembers() {
		_, err := cliInt.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

type changeWanStateRequest struct {
	name        string
	publisherId string
	state       codecTypes.WanReplicationState
}

func changeWanState(ctx context.Context, client *hazelcast.Client, request *changeWanStateRequest) error {
	cliInt := hazelcast.NewClientInternal(client)

	req := codec.EncodeMCChangeWanReplicationStateRequest(
		request.name,
		request.publisherId,
		request.state,
	)

	for _, member := range cliInt.OrderedMembers() {
		_, err := cliInt.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertAckType(ackType hazelcastcomv1alpha1.AcknowledgementType) int32 {
	switch ackType {
	case hazelcastcomv1alpha1.AckOnReceipt:
		return 0
	case hazelcastcomv1alpha1.AckOnOperationComplete:
		return 1
	default:
		return -1
	}
}

func convertQueueBehavior(behavior hazelcastcomv1alpha1.FullBehaviorSetting) int32 {
	switch behavior {
	case hazelcastcomv1alpha1.DiscardAfterMutation:
		return 0
	case hazelcastcomv1alpha1.ThrowException:
		return 1
	case hazelcastcomv1alpha1.ThrowExceptionOnlyIfReplicationActive:
		return 2
	default:
		return -1
	}
}

func insertLastAppliedSpec(wan *hazelcastcomv1alpha1.WanReplication) *hazelcastcomv1alpha1.WanReplication {
	b, _ := json.Marshal(wan.Spec)
	if wan.Annotations == nil {
		wan.Annotations = make(map[string]string)
	}
	wan.Annotations[n.LastAppliedSpecAnnotation] = string(b)
	return wan
}

func insertLastSuccessfullyAppliedSpec(wan *hazelcastcomv1alpha1.WanReplication) *hazelcastcomv1alpha1.WanReplication {
	b, _ := json.Marshal(wan.Spec)
	if wan.Annotations == nil {
		wan.Annotations = make(map[string]string)
	}
	wan.Annotations[n.LastSuccessfulSpecAnnotation] = string(b)
	return wan
}

type LogKey string

var ctxLogger = LogKey("logger")

func getLogger(ctx context.Context) logr.Logger {
	return ctx.Value(ctxLogger).(logr.Logger)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastcomv1alpha1.WanReplication{}).
		Complete(r)
}
