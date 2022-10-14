package hazelcast

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// MultiMapReconciler reconciles a MultiMap object
type MultiMapReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
}

func NewMultiMapReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}) *MultiMapReconciler {
	return &MultiMapReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps/finalizers,verbs=update

func (r *MultiMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-multimap", req.NamespacedName)
	mm := &hazelcastv1alpha1.MultiMap{}

	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, mm, r.Update, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	ms, err := r.ReconcileMultiMapConfig(ctx, mm, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, mm, dsPendingStatus(retryAfterForDataStructures).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateDSStatus(ctx, r.Client, mm, dsPersistingStatus(1*time.Second).withMessage("Persisting the applied multiMap config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateMultiMapConfigPersistence(ctx, mm)
	if err != nil {
		return updateDSStatus(ctx, r.Client, mm, dsFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, mm, dsPersistingStatus(1*time.Second).withMessage("Waiting for MultiMap Config to be persisted."))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, mm, logger)
}

func (r *MultiMapReconciler) ReconcileMultiMapConfig(
	ctx context.Context,
	mm *hazelcastv1alpha1.MultiMap,
	cl *hazelcast.Client,
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
	multiMapInput.Binary = mms.Binary
	multiMapInput.CollectionType = string(mms.CollectionType)
}

func (r *MultiMapReconciler) validateMultiMapConfigPersistence(ctx context.Context, mm *hazelcastv1alpha1.MultiMap) (bool, error) {
	hzConfig, err := getHazelcastConfigMap(ctx, r.Client, mm)
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
