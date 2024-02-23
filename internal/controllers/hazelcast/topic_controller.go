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
	recoptions "github.com/hazelcast/hazelcast-platform-operator/internal/controllers"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

// TopicReconciler reconciles a Topic object
type TopicReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
	clientRegistry   hzclient.ClientRegistry
}

func NewTopicReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cs hzclient.ClientRegistry) *TopicReconciler {
	return &TopicReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
		clientRegistry:   cs,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=topics,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=topics/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=topics/finalizers,verbs=update,namespace=watched

func (r *TopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-topic", req.NamespacedName)
	t := &hazelcastv1alpha1.Topic{}

	cl, res, err := initialSetupDS(ctx, r.Client, req.NamespacedName, t, r.Update, r.clientRegistry, logger)
	if cl == nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return res, nil
	}

	ms, err := r.ReconcileTopicConfig(ctx, t, cl, logger)
	if err != nil {
		return updateDSStatus(ctx, r.Client, t, recoptions.RetryAfter(retryAfterForDataStructures),
			withDSState(hazelcastv1alpha1.DataStructurePending),
			withDSMessage(err.Error()),
			withDSMemberStatuses(ms))
	}

	requeue, err := updateDSStatus(ctx, r.Client, t, recoptions.RetryAfter(1*time.Second),
		withDSState(hazelcastv1alpha1.DataStructurePersisting),
		withDSMessage("Persisting the applied multiMap config."),
		withDSMemberStatuses(ms))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateTopicConfigPersistence(ctx, t)
	if err != nil {
		return updateDSStatus(ctx, r.Client, t, recoptions.Error(err),
			withDSFailedState(err.Error()))
	}

	if !persisted {
		return updateDSStatus(ctx, r.Client, t, recoptions.RetryAfter(1*time.Second),
			withDSState(hazelcastv1alpha1.DataStructurePersisting),
			withDSMessage("Waiting for Cache Config to be persisted."),
			withDSMemberStatuses(ms))
	}

	return finalSetupDS(ctx, r.Client, r.phoneHomeTrigger, t, logger)
}

func (r *TopicReconciler) ReconcileTopicConfig(
	ctx context.Context,
	t *hazelcastv1alpha1.Topic,
	cl hzclient.Client,
	logger logr.Logger,
) (map[string]hazelcastv1alpha1.DataStructureConfigState, error) {
	var req *proto.ClientMessage

	topicInput := codecTypes.DefaultTopicConfigInput()
	fillTopicConfigInput(topicInput, t)

	req = codec.EncodeDynamicConfigAddTopicConfigRequest(topicInput)

	return sendCodecRequest(ctx, cl, t, req, logger)
}

func fillTopicConfigInput(topicInput *codecTypes.TopicConfig, t *hazelcastv1alpha1.Topic) {
	topicInput.Name = t.GetDSName()

	ts := t.Spec
	topicInput.GlobalOrderingEnabled = ts.GlobalOrderingEnabled
	topicInput.MultiThreadingEnabled = ts.MultiThreadingEnabled
}

func (r *TopicReconciler) validateTopicConfigPersistence(ctx context.Context, t *hazelcastv1alpha1.Topic) (bool, error) {
	hzConfig, err := getHazelcastConfig(ctx, r.Client, t)
	if err != nil {
		return false, err
	}

	tcfg, ok := hzConfig.Hazelcast.Topic[t.GetDSName()]
	if !ok {
		return false, nil
	}
	currentcfg := createTopicConfig(t)

	if !reflect.DeepEqual(tcfg, currentcfg) {
		return false, nil
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Topic{}).
		Complete(r)
}
