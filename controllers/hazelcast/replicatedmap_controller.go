package hazelcast

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	proto "github.com/hazelcast/hazelcast-go-client"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// ReplicatedMapReconciler reconciles a ReplicatedMap object
type ReplicatedMapReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewReplicatedMapReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cs hzclient.ClientRegistry) *ReplicatedMapReconciler {
	return &ReplicatedMapReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
		clientRegistry:   cs,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=replicatedmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=replicatedmaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=replicatedmaps/finalizers,verbs=update

func (r *ReplicatedMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-replicatedmap", req.NamespacedName)
	rm := &hazelcastv1alpha1.ReplicatedMap{}

	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, rm, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	ms, err := r.ReconcileReplicatedMapConfig(ctx, rm, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, rm, dsPendingStatus(retryAfterForDataStructures).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateDSStatus(ctx, r.Client, rm, dsPersistingStatus(1*time.Second).withMessage("Persisting the applied ReplicatedMap config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateReplicatedMapConfigPersistence(ctx, rm)
	if err != nil {
		return updateDSStatus(ctx, r.Client, rm, dsFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, rm, dsPersistingStatus(1*time.Second).withMessage("Waiting for ReplicatedMap Config to be persisted."))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, rm, logger)
}

func (r *ReplicatedMapReconciler) ReconcileReplicatedMapConfig(
	ctx context.Context,
	rm *hazelcastv1alpha1.ReplicatedMap,
	cl hzclient.Client,
	logger logr.Logger,
) (map[string]hazelcastv1alpha1.DataStructureConfigState, error) {
	var req *proto.ClientMessage

	replicatedMapInput := codecTypes.DefaultReplicatedMapConfigInput()
	fillReplicatedConfigInput(replicatedMapInput, rm)

	req = codec.EncodeDynamicConfigAddReplicatedMapConfigRequest(replicatedMapInput)

	return sendCodecRequest(ctx, cl, rm, req, logger)
}

func fillReplicatedConfigInput(replicatedMapInput *codecTypes.ReplicatedMapConfig, rm *hazelcastv1alpha1.ReplicatedMap) {
	replicatedMapInput.Name = rm.GetDSName()

	rms := rm.Spec
	replicatedMapInput.InMemoryFormat = string(rms.InMemoryFormat)
	replicatedMapInput.AsyncFillup = rms.AsyncFillup
}

func (r *ReplicatedMapReconciler) validateReplicatedMapConfigPersistence(ctx context.Context, rm *hazelcastv1alpha1.ReplicatedMap) (bool, error) {
	hzConfig, err := getHazelcastConfigMap(ctx, r.Client, rm)
	if err != nil {
		return false, err
	}

	rmcfg, ok := hzConfig.Hazelcast.ReplicatedMap[rm.GetDSName()]
	if !ok {
		return false, nil
	}
	currentRMcfg := createReplicatedMapConfig(rm)

	if !reflect.DeepEqual(rmcfg, currentRMcfg) {
		return false, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReplicatedMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.ReplicatedMap{}).
		Complete(r)
}
