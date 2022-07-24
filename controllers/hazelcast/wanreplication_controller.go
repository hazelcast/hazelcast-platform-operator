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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// WanReplicationReconciler reconciles a WanReplication object
type WanReplicationReconciler struct {
	client.Client
	logr.Logger
}

func NewWanReplicationReconciler(client client.Client, log logr.Logger) *WanReplicationReconciler {
	return &WanReplicationReconciler{
		Client: client,
		Logger: log,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=wanreplications/finalizers,verbs=update

func (r *WanReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	wan := &hazelcastcomv1alpha1.WanReplication{}
	if err := r.Get(ctx, req.NamespacedName, wan); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(util.DebugLevel).Info("Could not find WanReplication, it is probably already deleted")
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	ctx = context.WithValue(ctx, util.CtxLogger, logger)

	cli, err := r.getHazelcastClient(ctx, wan)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !wan.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(wan, n.Finalizer) {
			logger.Info("Deleting WAN configuration")
			if err := stopWanReplication(ctx, cli, wan); err != nil {
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
	if !controllerutil.ContainsFinalizer(wan, n.Finalizer) {
		controllerutil.AddFinalizer(wan, n.Finalizer)
		logger.Info("Adding finalizer")
		if err := r.Update(ctx, wan); err != nil {
			return ctrl.Result{}, err
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
		return updateWanStatus(ctx, r.Client, wan, wanFailedStatus().withMessage("WanReplicationSpec is not updatable"))
	}

	// Check publisherId is registered to the status, otherwise issue WanReplication to Hazelcast
	if wan.Status.PublisherId == "" {
		logger.Info("Applying WAN configuration")
		if publisherId, err := applyWanReplication(ctx, cli, wan); err != nil {
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

func hasUpdate(wan *hazelcastcomv1alpha1.WanReplication) (bool, error) {
	specStr, ok := wan.Annotations[n.LastAppliedSpecAnnotation]
	if !ok {
		return false, fmt.Errorf("last applied spec is not present")
	}
	lastSpec := &hazelcastcomv1alpha1.WanReplicationSpec{}
	err := json.Unmarshal([]byte(specStr), lastSpec)
	if err != nil {
		return false, fmt.Errorf("last applied spec is not properly formatted")
	}
	return !reflect.DeepEqual(&wan.Spec, lastSpec), nil
}

func (r *WanReplicationReconciler) getHazelcastClient(ctx context.Context, wan *hazelcastcomv1alpha1.WanReplication) (*hazelcast.Client, error) {
	m := &hazelcastcomv1alpha1.Map{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: wan.Spec.MapResourceName, Namespace: wan.Namespace}, m); err != nil {
		return nil, fmt.Errorf("failed to get Map CR from WanReplication: %w", err)
	}
	return GetHazelcastClient(m)
}

func hazelcastWanReplicationName(mapName string) string {
	return mapName + "-default"
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

func isApplied(wan *hazelcastcomv1alpha1.WanReplication) bool {
	_, ok := wan.Annotations[n.LastAppliedSpecAnnotation]
	return ok
}

func isSuccessfullyApplied(wan *hazelcastcomv1alpha1.WanReplication) bool {
	_, ok := wan.Annotations[n.LastSuccessfulSpecAnnotation]
	return ok
}

func insertLastAppliedSpec(wan *hazelcastcomv1alpha1.WanReplication) *hazelcastcomv1alpha1.WanReplication {
	b, _ := json.Marshal(wan.Spec)
	if wan.Annotations == nil {
		wan.Annotations = make(map[string]string)
	}
	wan.Annotations[n.LastAppliedSpecAnnotation] = string(b)
	return wan
}

func insertLastSuccessfullyAppliedSpec(wan *hazelcastcomv1alpha1.WanReplication) *hazelcastcomv1alpha1.WanReplication {
	b, _ := json.Marshal(wan.Spec)
	if wan.Annotations == nil {
		wan.Annotations = make(map[string]string)
	}
	wan.Annotations[n.LastSuccessfulSpecAnnotation] = string(b)
	return wan
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastcomv1alpha1.WanReplication{}).
		Complete(r)
}
