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
	recoptions "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// MultiMapReconciler reconciles a MultiMap object
type MultiMapReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewMultiMapReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cs hzclient.ClientRegistry) *MultiMapReconciler {
	return &MultiMapReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
		clientRegistry:   cs,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps/finalizers,verbs=update,namespace=watched

func (r *MultiMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-multimap", req.NamespacedName)
	mm := &hazelcastv1alpha1.MultiMap{}

	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, mm, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	ms, err := r.ReconcileMultiMapConfig(ctx, mm, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, mm, recoptions.RetryAfter(retryAfterForDataStructures),
			withDSState(hazelcastv1alpha1.DataStructurePending),
			withDSMessage(err.Error()),
			withDSMemberStatuses(ms))
	}

	requeue, err := updateDSStatus(ctx, r.Client, mm, recoptions.RetryAfter(1*time.Second),
		withDSState(hazelcastv1alpha1.DataStructurePersisting),
		withDSMessage("Persisting the applied MultiMap config."),
		withDSMemberStatuses(ms))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateMultiMapConfigPersistence(ctx, mm)
	if err != nil {
		return updateDSStatus(ctx, r.Client, mm, recoptions.Error(err),
			withDSFailedState(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, mm, recoptions.RetryAfter(1*time.Second),
			withDSState(hazelcastv1alpha1.DataStructurePersisting),
			withDSMessage("Waiting for MultiMap Config to be persisted."),
			withDSMemberStatuses(ms))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, mm, logger)
}

func (r *MultiMapReconciler) ReconcileMultiMapConfig(
	ctx context.Context,
	mm *hazelcastv1alpha1.MultiMap,
	cl hzclient.Client,
	logger logr.Logger,
) (map[string]hazelcastv1alpha1.DataStructureConfigState, error) {
	var req *proto.ClientMessage

	multiMapInput := codecTypes.DefaultMultiMapConfigInput()
	fillMultiMapConfigInput(multiMapInput, mm)

	req = codec.EncodeDynamicConfigAddMultiMapConfigRequest(multiMapInput)

	return sendCodecRequest(ctx, cl, mm, req, logger)
}

func fillMultiMapConfigInput(multiMapInput *codecTypes.MultiMapConfig, mm *hazelcastv1alpha1.MultiMap) {
	multiMapInput.Name = mm.GetDSName()

	mms := mm.Spec
	multiMapInput.BackupCount = *mms.BackupCount
	multiMapInput.AsyncBackupCount = mms.AsyncBackupCount
	multiMapInput.Binary = mms.Binary
	multiMapInput.CollectionType = string(mms.CollectionType)
}

func (r *MultiMapReconciler) validateMultiMapConfigPersistence(ctx context.Context, mm *hazelcastv1alpha1.MultiMap) (bool, error) {
	hzConfig, err := getHazelcastConfig(ctx, r.Client, getHzNamespacedName(mm))
	if err != nil {
		return false, err
	}

	mmcfg, ok := hzConfig.Hazelcast.MultiMap[mm.GetDSName()]
	if !ok {
		return false, nil
	}
	currentMMcfg := createMultiMapConfig(mm)

	if !reflect.DeepEqual(mmcfg, currentMMcfg) {
		return false, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.MultiMap{}).
		Complete(r)
}
