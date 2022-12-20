package hazelcast

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// CacheReconciler reconciles a Cache object
type CacheReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewCacheReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cr *hzclient.HazelcastClientRegistry) *CacheReconciler {
	return &CacheReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
		clientRegistry:   cr,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=caches,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=caches/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=caches/finalizers,verbs=update

func (r *CacheReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-cache", req.NamespacedName)

	c := &hazelcastv1alpha1.Cache{}
	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, c, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	err = r.validateCachePersistence(ctx, req, c)
	if err != nil {
		return updateDSStatus(ctx, r.Client, c, dsFailedStatus(err).
			withMessage(err.Error()))
	}

	ms, err := r.ReconcileCacheConfig(ctx, c, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, c, dsPendingStatus(retryAfterForDataStructures).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateDSStatus(ctx, r.Client, c, dsPersistingStatus(1*time.Second).withMessage("Persisting the applied multiMap config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateCacheConfigPersistence(ctx, c)
	if err != nil {
		return updateDSStatus(ctx, r.Client, c, dsFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, c, dsPersistingStatus(1*time.Second).withMessage("Waiting for Cache Config to be persisted."))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, c, logger)
}

func (r *CacheReconciler) validateCachePersistence(ctx context.Context, req ctrl.Request, c *hazelcastv1alpha1.Cache) error {
	h := &hazelcastv1alpha1.Hazelcast{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: c.Spec.HazelcastResourceName}, h)
	if err != nil {
		return fmt.Errorf("could not create/update Cache config: Hazelcast resource not found: %w", err)
	}
	err = util.ValidatePersistence(c.Spec.PersistenceEnabled, h)
	if err != nil {
		return err
	}
	return nil
}

func (r *CacheReconciler) ReconcileCacheConfig(
	ctx context.Context,
	c *hazelcastv1alpha1.Cache,
	cl hzclient.Client,
	logger logr.Logger,
) (map[string]hazelcastv1alpha1.DataStructureConfigState, error) {
	var req *hazelcast.ClientMessage

	cacheInput := codecTypes.DefaultCacheConfigInput()
	fillCacheConfigInput(cacheInput, c)

	req = codec.EncodeDynamicConfigAddCacheConfigRequest(cacheInput)

	return sendCodecRequest(ctx, cl, c, req, logger)
}

func fillCacheConfigInput(cacheInput *codecTypes.CacheConfigInput, c *hazelcastv1alpha1.Cache) {
	cacheInput.Name = c.GetDSName()
	cs := c.Spec
	cacheInput.BackupCount = *cs.BackupCount
	cacheInput.AsyncBackupCount = cs.AsyncBackupCount
	cacheInput.KeyType = cs.KeyType
	cacheInput.ValueType = cs.ValueType
	cacheInput.HotRestartConfig.Enabled = cs.PersistenceEnabled
}

func (r *CacheReconciler) validateCacheConfigPersistence(ctx context.Context, c *hazelcastv1alpha1.Cache) (bool, error) {
	hzConfig, err := getHazelcastConfigMap(ctx, r.Client, c)
	if err != nil {
		return false, err
	}

	ccfg, ok := hzConfig.Hazelcast.Cache[c.GetDSName()]
	if !ok {
		return false, nil
	}
	currentQCfg := createCacheConfig(c)

	if !reflect.DeepEqual(ccfg, currentQCfg) {
		return false, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CacheReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Cache{}).
		Complete(r)
}
