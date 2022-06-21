package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// WanConfigurationReconciler reconciles a WanConfiguration object
type WanConfigurationReconciler struct {
	client.Client
	logr.Logger
}

func NewWanConfigurationReconciler(client client.Client, log logr.Logger) *WanConfigurationReconciler {
	return &WanConfigurationReconciler{
		Client: client,
		Logger: log,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wanconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanconfigurations/finalizers,verbs=update

func (r *WanConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	wan := &hazelcastcomv1alpha1.WanConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, wan); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(2).Info("Could not find WanConfiguration, it is probably already deleted")
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	ctx = context.WithValue(ctx, LogKey("logger"), logger)

	cli, err := r.getHazelcastClient(ctx, wan)
	if err != nil {
		return ctrl.Result{}, err
	}

	if wan.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(wan, n.Finalizer) {
			controllerutil.AddFinalizer(wan, n.Finalizer)
			logger.Info("Adding finalizer")
			if err := r.Update(ctx, wan); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else {
		if controllerutil.ContainsFinalizer(wan, n.Finalizer) {
			logger.Info("Deleting WAN configuration")
			if err := r.stopWanConfiguration(ctx, cli, wan); err != nil {
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

	if !isApplied(wan) {
		if err := r.Update(ctx, insertLastAppliedSpec(wan)); err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		} else {
			return updateWanStatus(ctx, r.Client, wan, wanPendingStatus())
		}
	}

	updated, err := hasUpdate(wan)
	if err != nil {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
	}
	if updated {
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage("WanConfiguration spec is not updatable"))
	}

	// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
	if wan.Status.PublisherId == "" {
		logger.Info("Applying WAN configuration")
		if publisherId, err := r.applyWanConfiguration(ctx, cli, wan); err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		} else {
			return updateWanStatus(ctx, r.Client, wan, wanPendingStatus().withPublisherId(publisherId))
		}
	}

	if !isSuccessfullyApplied(wan) {
		if err := r.Update(ctx, insertLastSuccessfullyAppliedSpec(wan)); err != nil {
			return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage(err.Error()))
		}
	}

	return updateWanStatus(ctx, r.Client, wan, wanSuccessStatus().withPublisherId(wan.Status.PublisherId))
}

func hasUpdate(wan *hazelcastcomv1alpha1.WanConfiguration) (bool, error) {
	specStr, ok := wan.Annotations[n.LastAppliedSpecAnnotation]
	if !ok {
		return false, fmt.Errorf("last applied spec is not present")
	}
	lastSpec := &hazelcastcomv1alpha1.WanConfiguration{}
	err := json.Unmarshal([]byte(specStr), lastSpec)
	if err != nil {
		return false, fmt.Errorf("last applied spec is not properly formatted")
	}
	return !reflect.DeepEqual(wan.Spec, lastSpec), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastcomv1alpha1.WanConfiguration{}).
		Complete(r)
}

func (r *WanConfigurationReconciler) getHazelcastClient(ctx context.Context, wan *hazelcastcomv1alpha1.WanConfiguration) (*hazelcast.Client, error) {
	m := &hazelcastcomv1alpha1.Map{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: wan.Spec.MapResourceName, Namespace: wan.Namespace}, m); err != nil {
		return nil, fmt.Errorf("failed to get Map CR from WanConfiguration: %w", err)
	}
	return GetHazelcastClient(m)
}

func (r *WanConfigurationReconciler) applyWanConfiguration(ctx context.Context, client *hazelcast.Client, wan *hazelcastcomv1alpha1.WanConfiguration) (string, error) {
	publisherId := wan.Name + "-" + rand.String(16)

	req := &addBatchPublisherRequest{
		hazelcastWanConfigurationName(wan.Spec.MapResourceName),
		wan.Spec.TargetClusterName,
		wan.Status.PublisherId,
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

func (r *WanConfigurationReconciler) stopWanConfiguration(ctx context.Context, client *hazelcast.Client, wan *hazelcastcomv1alpha1.WanConfiguration) error {
	log := getLogger(ctx)
	if wan.Status.PublisherId == "" {
		log.V(2).Info("publisherId is empty, will skip stopping WAN replication")
		return nil
	}

	req := &changeWanStateRequest{
		name:        hazelcastWanConfigurationName(wan.Spec.MapResourceName),
		publisherId: wan.Status.PublisherId,
		state:       codecTypes.WanReplicationStateStopped,
	}
	return changeWanState(ctx, client, req)
}

func hazelcastWanConfigurationName(mapName string) string {
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

func isApplied(wan *hazelcastcomv1alpha1.WanConfiguration) bool {
	_, ok := wan.Annotations[n.LastAppliedSpecAnnotation]
	return ok
}

func isSuccessfullyApplied(wan *hazelcastcomv1alpha1.WanConfiguration) bool {
	_, ok := wan.Annotations[n.LastSuccessfulSpecAnnotation]
	return ok
}

func insertLastAppliedSpec(wan *hazelcastcomv1alpha1.WanConfiguration) *hazelcastcomv1alpha1.WanConfiguration {
	b, _ := json.Marshal(wan)
	wan.Annotations[n.LastAppliedSpecAnnotation] = string(b)
	return wan
}

func insertLastSuccessfullyAppliedSpec(wan *hazelcastcomv1alpha1.WanConfiguration) *hazelcastcomv1alpha1.WanConfiguration {
	b, _ := json.Marshal(wan)
	wan.Annotations[n.LastSuccessfulSpecAnnotation] = string(b)
	return wan
}

type LogKey string

var ctxLogger = LogKey("logger")

func getLogger(ctx context.Context) logr.Logger {
	return ctx.Value(ctxLogger).(logr.Logger)
}
