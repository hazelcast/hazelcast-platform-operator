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

	q := &hazelcastv1alpha1.Cache{}
	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, q, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	ms, err := r.ReconcileCacheConfig(ctx, q, cl, logger)
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

	persisted, err := r.validateCacheConfigPersistence(ctx, q)
	if err != nil {
		return updateDSStatus(ctx, r.Client, q, dsFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, q, dsPersistingStatus(1*time.Second).withMessage("Waiting for Cache Config to be persisted."))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, q, logger)
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
	cacheInput.AsyncBackupCount = *cs.AsyncBackupCount
	cacheInput.KeyType = cs.KeyType
	cacheInput.ValueType = cs.ValueType

	// TODO: Temporary solution for https://github.com/hazelcast/hazelcast/issues/21799
	cacheInput.DataPersistenceConfig = codecTypes.DataPersistenceConfig{
		Enabled: false,
		Fsync:   true,
	}
	cacheInput.MerkleTreeConfig = codecTypes.MerkleTreeConfig{
		IsDefined:  false,
		Enabled:    false,
		Depth:      2,
		EnabledSet: false,
	}
	cacheInput.HotRestartConfig = codecTypes.HotRestartConfig{
		IsDefined: false,
		Enabled:   false,
		Fsync:     true,
	}
	cacheInput.EventJournalConfig = codecTypes.EventJournalConfig{
		IsDefined:         false,
		Enabled:           false,
		Capacity:          1,
		TimeToLiveSeconds: 1,
	}
	//default values
	cacheInput.EvictionConfig = codecTypes.EvictionConfigHolder{
		Size:           10000,
		MaxSizePolicy:  "ENTRY_COUNT",
		EvictionPolicy: "LRU",
	}
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
