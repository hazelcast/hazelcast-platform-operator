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
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
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

const retryAfterForMultiMap = 5 * time.Second

//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=multimaps/finalizers,verbs=update

func (r *MultiMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-multimap", req.NamespacedName)
	mm := &hazelcastv1alpha1.MultiMap{}
	err := r.Client.Get(ctx, req.NamespacedName, mm)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("MultiMap resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get MultiMap: %w", err)
	}

	err = r.addFinalizer(ctx, mm, logger)
	if err != nil {
		return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).withMessage(err.Error()))
	}

	if mm.GetDeletionTimestamp() != nil {
		updateMultiMapStatus(ctx, r.Client, mm, mmTerminatingStatus(nil)) //nolint:errcheck
		err = r.executeFinalizer(ctx, mm, logger)
		if err != nil {
			return updateMultiMapStatus(ctx, r.Client, mm, mmTerminatingStatus(err).withMessage(err.Error()))
		}
		logger.V(2).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	s, createdBefore := mm.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]

	if createdBefore {
		mms, err := json.Marshal(mm.Spec)

		if err != nil {
			err = fmt.Errorf("error marshaling MultiMap as JSON: %w", err)
			return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).withMessage(err.Error()))
		}
		if s == string(mms) {
			logger.Info("MultiMap Config was already applied.", "name", mm.Name, "namespace", mm.Namespace)
			return updateMultiMapStatus(ctx, r.Client, mm, mmSuccessStatus())
		}

		lastSpec := &hazelcastv1alpha1.MultiMapSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last Map Spec: %w", err)
			return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).withMessage(err.Error()))
		}
		if !reflect.DeepEqual(&mm.Spec, lastSpec) {
			return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(fmt.Errorf("MultiMapSpec is not updatable")))
		}
	}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: mm.Spec.HazelcastResourceName}, h)
	if err != nil {
		err = fmt.Errorf("could not create/update MultiMap config: Hazelcast resource not found: %w", err)
		return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).withMessage(err.Error()))
	}
	if h.Status.Phase != hazelcastv1alpha1.Running {
		err = errors.NewServiceUnavailable("Hazelcast CR is not ready")
		return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).withMessage(err.Error()))
	}

	cl, err := GetMMHazelcastClient(mm)
	if err != nil {
		if errors.IsInternalError(err) {
			return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).
				withMessage(err.Error()))
		}
		return updateMultiMapStatus(ctx, r.Client, mm, mmPendingStatus(retryAfterForMultiMap).
			withMessage(err.Error()))
	}

	if mm.Status.State != hazelcastv1alpha1.MultiMapPersisting {
		requeue, err := updateMultiMapStatus(ctx, r.Client, mm, mmPendingStatus(0).withMessage("Applying new multiMap configuration."))
		if err != nil {
			return requeue, err
		}
	}

	ms, err := r.ReconcileMultiMapConfig(ctx, mm, cl)
	if err != nil {
		return updateMultiMapStatus(ctx, r.Client, mm, mmPendingStatus(retryAfterForMultiMap).
			withError(err).
			withMessage(err.Error()).
			withMemberStatuses(ms))
	}

	requeue, err := updateMultiMapStatus(ctx, r.Client, mm, mmPersistingStatus(1*time.Second).withMessage("Persisting the applied multiMap config."))
	if err != nil {
		return requeue, err
	}

	persisted, err := r.validateMultiMapConfigPersistence(ctx, mm)
	if err != nil {
		return updateMultiMapStatus(ctx, r.Client, mm, mmFailedStatus(err).withMessage(err.Error()))
	}

	if !persisted {
		return updateMultiMapStatus(ctx, r.Client, mm, mmPersistingStatus(1*time.Second).withMessage("Waiting for MultiMap Config to be persisted."))
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(mm) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, mm)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	return updateMultiMapStatus(ctx, r.Client, mm, mmSuccessStatus().
		withMemberStatuses(nil))
}

func GetMMHazelcastClient(mm *hazelcastv1alpha1.MultiMap) (*hazelcast.Client, error) {
	hzcl, ok := hzclient.GetClient(types.NamespacedName{Name: mm.Spec.HazelcastResourceName, Namespace: mm.Namespace})
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("cannot connect to the cluster for %s", mm.Spec.HazelcastResourceName))
	}
	if hzcl.Client == nil || !hzcl.Client.Running() {
		return nil, fmt.Errorf("trying to connect to the cluster %s", mm.Spec.HazelcastResourceName)
	}

	return hzcl.Client, nil
}

func (r *MultiMapReconciler) addFinalizer(ctx context.Context, mm *hazelcastv1alpha1.MultiMap, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(mm, n.Finalizer) && mm.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(mm, n.Finalizer)
		err := r.Update(ctx, mm)
		if err != nil {
			return err
		}
		logger.V(util.DebugLevel).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *MultiMapReconciler) executeFinalizer(ctx context.Context, mm *hazelcastv1alpha1.MultiMap, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(mm, n.Finalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(mm, n.Finalizer)
	err := r.Update(ctx, mm)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *MultiMapReconciler) ReconcileMultiMapConfig(
	ctx context.Context,
	mm *hazelcastv1alpha1.MultiMap,
	cl *hazelcast.Client,
) (map[string]hazelcastv1alpha1.MultiMapConfigState, error) {
	ci := hazelcast.NewClientInternal(cl)
	var req *proto.ClientMessage

	multiMapInput := codecTypes.DefaultMultiMapConfigInput()
	fillMultiMapConfigInput(multiMapInput, mm)

	req = codec.EncodeDynamicConfigAddMultiMapConfigRequest(multiMapInput)

	memberStatuses := map[string]hazelcastv1alpha1.MultiMapConfigState{}
	var failedMembers strings.Builder
	for _, member := range ci.OrderedMembers() {
		if status, ok := mm.Status.MemberStatuses[member.UUID.String()]; ok && status == hazelcastv1alpha1.MultiMapSuccess {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MultiMapSuccess
			continue
		}
		_, err := ci.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MultiMapFailed
			failedMembers.WriteString(member.UUID.String() + ", ")
			r.Log.Error(err, "Failed with member")
			continue
		}
		memberStatuses[member.UUID.String()] = hazelcastv1alpha1.MultiMapSuccess
	}
	errString := failedMembers.String()
	if errString != "" {
		return memberStatuses, fmt.Errorf("error creating/updating the MultiMap config %s for members %s", mm.MultiMapName(), errString[:len(errString)-2])
	}

	return memberStatuses, nil
}

func fillMultiMapConfigInput(multiMapInput *codecTypes.MultiMapConfig, mm *hazelcastv1alpha1.MultiMap) {
	multiMapInput.Name = mm.MultiMapName()

	mms := mm.Spec
	multiMapInput.BackupCount = *mms.BackupCount
	multiMapInput.Binary = mms.Binary
	multiMapInput.CollectionType = string(mms.CollectionType)
}

func (r *MultiMapReconciler) validateMultiMapConfigPersistence(ctx context.Context, mm *hazelcastv1alpha1.MultiMap) (bool, error) {
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: mm.Spec.HazelcastResourceName, Namespace: mm.Namespace}, cm)
	if err != nil {
		return false, fmt.Errorf("could not find ConfigMap for multiMap config persistence")
	}

	hzConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal([]byte(cm.Data["hazelcast.yaml"]), hzConfig)
	if err != nil {
		return false, fmt.Errorf("persisted ConfigMap is not formatted correctly")
	}

	mmcfg, ok := hzConfig.Hazelcast.MultiMap[mm.MultiMapName()]
	if !ok {
		return false, nil
	}
	currentMMcfg := createMultiMapConfig(mm)

	if !reflect.DeepEqual(mmcfg, currentMMcfg) {
		return false, nil
	}
	return true, nil
}

func (r *MultiMapReconciler) updateLastSuccessfulConfiguration(ctx context.Context, mm *hazelcastv1alpha1.MultiMap) error {
	mms, err := json.Marshal(mm.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, mm, func() error {
		if mm.ObjectMeta.Annotations == nil {
			mm.ObjectMeta.Annotations = map[string]string{}
		}
		mm.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(mms)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		r.Log.Info("Operation result", "MultiMap Annotation", mm.Name, "result", opResult)
	}
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.MultiMap{}).
		Complete(r)
}
