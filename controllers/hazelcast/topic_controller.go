package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/client"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/validation"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// TopicReconciler reconciles a Topic object
type TopicReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
}

func NewTopicReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}) *TopicReconciler {
	return &TopicReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
	}
}

const retryAfterForTopic = 5 * time.Second

//+kubebuilder:rbac:groups=hazelcast.com,resources=topics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=topics/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=topics/finalizers,verbs=update

func (r *TopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-topic", req.NamespacedName)
	t := &hazelcastv1alpha1.Topic{}

	err := r.Client.Get(ctx, req.NamespacedName, t)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Topic resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get Topic: %w", err)
	}

	err = r.addFinalizer(ctx, t, logger)
	if err != nil {
		return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
	}

	if t.GetDeletionTimestamp() != nil {
		updateTopicStatus(ctx, r.Client, t, topicTerminatingStatus(nil)) //nolint:errcheck
		err = r.executeFinalizer(ctx, t, logger)
		if err != nil {
			return updateTopicStatus(ctx, r.Client, t, topicTerminatingStatus(err).withMessage(err.Error()))
		}
		logger.V(2).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	err = validation.ValidateTopicSpec(t)
	if err != nil {
		return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
	}

	s, createdBefore := t.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]

	if createdBefore {
		ts, err := json.Marshal(t.Spec)

		if err != nil {
			err = fmt.Errorf("error marshaling Topic as JSON: %w", err)
			return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
		}
		if s == string(ts) {
			logger.Info("Topic Config was already applied.", "name", t.Name, "namespace", t.Namespace)
			return updateTopicStatus(ctx, r.Client, t, topicSuccessStatus())
		}

		lastSpec := &hazelcastv1alpha1.TopicSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last Topic Spec: %w", err)
			return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
		}
		if !reflect.DeepEqual(&t.Spec, lastSpec) {
			return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(fmt.Errorf("TopicSpec is not updatable")))
		}
	}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: t.Spec.HazelcastResourceName}, h)
	if err != nil {
		err = fmt.Errorf("could not create/update Topic config: Hazelcast resource not found: %w", err)
		return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
	}
	if h.Status.Phase != hazelcastv1alpha1.Running {
		err = errors.NewServiceUnavailable("Hazelcast CR is not ready")
		return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
	}

	cl, err := GetTopicHazelcastClient(t)
	if err != nil {
		if errors.IsInternalError(err) {
			return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).
				withMessage(err.Error()))
		}
		return updateTopicStatus(ctx, r.Client, t, topicPendingStatus(retryAfterForTopic).
			withMessage(err.Error()))
	}

	if t.Status.State != hazelcastv1alpha1.TopicPersisting {
		requeue, err := updateTopicStatus(ctx, r.Client, t, topicPendingStatus(0).withMessage("Applying new topic configuration."))
		if err != nil {
			return requeue, err
		}
	}

	ms, err := r.ReconcileTopicConfig(ctx, t, cl)
	if err != nil {
		return updateTopicStatus(ctx, r.Client, t, topicPendingStatus(retryAfterForTopic).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateTopicStatus(ctx, r.Client, t, topicPersistingStatus(1*time.Second).withMessage("Persisting the applied topic config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateTopicConfigPersistence(ctx, t)
	if err != nil {
		return updateTopicStatus(ctx, r.Client, t, topicFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateTopicStatus(ctx, r.Client, t, topicPersistingStatus(1*time.Second).withMessage("Waiting for Topic Config to be persisted."))
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(t) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, t)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	return updateTopicStatus(ctx, r.Client, t, topicSuccessStatus().
		withMemberStatuses(nil))
}

func (r *TopicReconciler) addFinalizer(ctx context.Context, t *hazelcastv1alpha1.Topic, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(t, n.Finalizer) && t.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(t, n.Finalizer)
		err := r.Update(ctx, t)
		if err != nil {
			return err
		}
		logger.V(util.DebugLevel).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *TopicReconciler) executeFinalizer(ctx context.Context, t *hazelcastv1alpha1.Topic, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(t, n.Finalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(t, n.Finalizer)
	err := r.Update(ctx, t)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func GetTopicHazelcastClient(t *hazelcastv1alpha1.Topic) (*hazelcast.Client, error) {
	hzcl, ok := hzclient.GetClient(types.NamespacedName{Name: t.Spec.HazelcastResourceName, Namespace: t.Namespace})
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("cannot connect to the cluster for %s", t.Spec.HazelcastResourceName))
	}
	if hzcl.Client == nil || !hzcl.Client.Running() {
		return nil, fmt.Errorf("trying to connect to the cluster %s", t.Spec.HazelcastResourceName)
	}

	return hzcl.Client, nil
}

func (r *TopicReconciler) ReconcileTopicConfig(
	ctx context.Context,
	t *hazelcastv1alpha1.Topic,
	cl *hazelcast.Client,
) (map[string]hazelcastv1alpha1.TopicConfigState, error) {
	ci := hazelcast.NewClientInternal(cl)
	var req *proto.ClientMessage

	topicInput := codecTypes.DefaultTopicConfigInput()
	fillTopicConfigInput(topicInput, t)

	req = codec.EncodeDynamicConfigAddTopicConfigRequest(topicInput)

	memberStatuses := map[string]hazelcastv1alpha1.TopicConfigState{}
	var failedMembers strings.Builder
	for _, member := range ci.OrderedMembers() {
		if status, ok := t.Status.MemberStatuses[member.UUID.String()]; ok && status == hazelcastv1alpha1.TopicSuccess {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.TopicSuccess
			continue
		}
		_, err := ci.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.TopicFailed
			failedMembers.WriteString(member.UUID.String() + ", ")
			r.Log.Error(err, "Failed with member")
			continue
		}
		memberStatuses[member.UUID.String()] = hazelcastv1alpha1.TopicSuccess
	}
	errString := failedMembers.String()
	if errString != "" {
		return memberStatuses, fmt.Errorf("error creating/updating the Topic config %s for members %s", t.TopicName(), errString[:len(errString)-2])
	}

	return memberStatuses, nil
}

func fillTopicConfigInput(topicInput *codecTypes.TopicConfig, t *hazelcastv1alpha1.Topic) {
	topicInput.Name = t.TopicName()

	ts := t.Spec
	topicInput.GlobalOrderingEnabled = ts.GlobalOrderingEnabled
	topicInput.MultiThreadingEnabled = ts.MultiThreadingEnabled
}

func (r *TopicReconciler) validateTopicConfigPersistence(ctx context.Context, t *hazelcastv1alpha1.Topic) (bool, error) {
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: t.Spec.HazelcastResourceName, Namespace: t.Namespace}, cm)
	if err != nil {
		return false, fmt.Errorf("could not find ConfigMap for topic config persistence")
	}

	hzConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal([]byte(cm.Data["hazelcast.yaml"]), hzConfig)
	if err != nil {
		return false, fmt.Errorf("persisted ConfigMap is not formatted correctly")
	}

	tcfg, ok := hzConfig.Hazelcast.Topic[t.TopicName()]
	if !ok {
		return false, nil
	}
	currentcfg := createTopicConfig(t)

	if !reflect.DeepEqual(tcfg, currentcfg) {
		return false, nil
	}
	return true, nil
}

func (r *TopicReconciler) updateLastSuccessfulConfiguration(ctx context.Context, t *hazelcastv1alpha1.Topic) error {
	topics, err := json.Marshal(t.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, t, func() error {
		if t.ObjectMeta.Annotations == nil {
			t.ObjectMeta.Annotations = map[string]string{}
		}
		t.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(topics)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		r.Log.Info("Operation result", "Topic Annotation", t.Name, "result", opResult)
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *TopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Topic{}).
		Complete(r)
}
