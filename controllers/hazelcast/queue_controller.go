package hazelcast

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// QueueReconciler reconciles a Queue object
type QueueReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewQueueReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cr hzclient.ClientRegistry) *QueueReconciler {
	return &QueueReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
		clientRegistry:   cr,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=queues,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=queues/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=queues/finalizers,verbs=update

func (r *QueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-queue", req.NamespacedName)

	q := &hazelcastv1alpha1.Queue{}
	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, q, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	ms, err := r.ReconcileQueueConfig(ctx, q, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, q, dsPendingStatus(retryAfterForDataStructures).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateDSStatus(ctx, r.Client, q, dsPersistingStatus(1*time.Second).withMessage("Persisting the applied multiMap config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateQueueConfigPersistence(ctx, q)
	if err != nil {
		return updateDSStatus(ctx, r.Client, q, dsFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, q, dsPersistingStatus(1*time.Second).withMessage("Waiting for Queue Config to be persisted."))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, q, logger)
}

func (r *QueueReconciler) validateQueueConfigPersistence(ctx context.Context, q *hazelcastv1alpha1.Queue) (bool, error) {
	hzConfig, err := getHazelcastConfigMap(ctx, r.Client, q)
	if err != nil {
		return false, err
	}

	qcfg, ok := hzConfig.Hazelcast.Queue[q.GetDSName()]
	if !ok {
		return false, nil
	}
	currentQCfg := createQueueConfig(q)

	if !reflect.DeepEqual(qcfg, currentQCfg) {
		return false, nil
	}
	return true, nil
}

func (r *QueueReconciler) ReconcileQueueConfig(
	ctx context.Context,
	q *hazelcastv1alpha1.Queue,
	cl hzclient.Client,
	logger logr.Logger,
) (map[string]hazelcastv1alpha1.DataStructureConfigState, error) {
	var req *hazelcast.ClientMessage

	queueInput := codecTypes.DefaultQueueConfigInput()
	fillQueueConfigInput(queueInput, q)

	req = codec.EncodeDynamicConfigAddQueueConfigRequest(queueInput)

	return sendCodecRequest(ctx, cl, q, req, logger)
}

func fillQueueConfigInput(queueInput *codecTypes.QueueConfigInput, q *hazelcastv1alpha1.Queue) {
	queueInput.Name = q.GetDSName()
	qs := q.Spec
	queueInput.BackupCount = *qs.BackupCount
	queueInput.AsyncBackupCount = *qs.AsyncBackupCount
	queueInput.EmptyQueueTtl = *qs.EmptyQueueTtlSeconds
	queueInput.MaxSize = *qs.MaxSize
	queueInput.PriorityComparatorClassName = qs.PriorityComparatorClassName
}

// SetupWithManager sets up the controller with the Manager.
func (r *QueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Queue{}).
		Complete(r)
}
