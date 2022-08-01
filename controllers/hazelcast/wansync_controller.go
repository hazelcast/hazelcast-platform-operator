package hazelcast

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// WanSyncReconciler reconciles a WanSync object
type WanSyncReconciler struct {
	client.Client
	logr.Logger
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wansyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=wansyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=wansyncs/finalizers,verbs=update

func (r *WanSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	wan := &hazelcastcomv1alpha1.WanSync{}
	if err := r.Get(ctx, req.NamespacedName, wan); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(util.DebugLevel).Info("Could not find WanSync, it is probably already deleted")
			return ctrl.Result{}, nil
		} else {
			return updateWanSyncStatus(ctx, r.Client, wan,
				wanSyncFailedStatus(fmt.Errorf("could not get WanSync: %w", err)))
		}
	}
	ctx = context.WithValue(ctx, util.CtxLogger, logger)

	m, err := r.getWanMap(ctx, wan)
	if err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan,
			wanSyncFailedStatus(fmt.Errorf("unable to get Hazelcast Map for WAN Sync: %w", err)))
	}
	cli, err := GetHazelcastClient(m)
	if err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan,
			wanSyncFailedStatus(fmt.Errorf("unable to create Hazelcast client: %w", err)))
	}

	if !wan.GetDeletionTimestamp().IsZero() {
		return r.executeFinalizer(ctx, wan, cli)
	}

	if util.IsSuccessfullyApplied(wan.ObjectMeta) {
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(wan, n.Finalizer) {
		controllerutil.AddFinalizer(wan, n.Finalizer)
		logger.Info("Adding finalizer")
		if err := r.Update(ctx, wan); err != nil {
			return updateWanSyncStatus(ctx, r.Client, wan,
				wanSyncFailedStatus(fmt.Errorf("failed to create add finalizer: %w", err)))
		}
	}

	if !util.IsApplied(wan.ObjectMeta) {
		if err := r.Update(ctx, util.InsertLastAppliedSpec(wan.Spec, wan)); err != nil {
			return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
		} else {
			return updateWanSyncStatus(ctx, r.Client, wan, wanSyncPendingStatus())
		}
	}

	// Check publisherId is registered to the status, otherwise issue WanReplication config to Hazelcast
	if wan.Status.PublisherId == "" {
		logger.Info("Applying WAN configuration")
		publisherId, err := r.getWanPublisherId(ctx, cli, wan)
		if err != nil {
			return updateWanSyncStatus(ctx, r.Client, wan,
				wanSyncFailedStatus(fmt.Errorf("failed to create WAN publisher: %w", err)))
		}
		if publisherId == "" {
			return updateWanSyncStatus(ctx, r.Client, wan,
				wanSyncFailedStatus(fmt.Errorf("publisherId is empty")))
		}
		logger.V(util.DebugLevel).Info("Applied the wan replication publisher",
			"WanSync", req.NamespacedName, "publisherId", publisherId)

		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncPendingStatus().withPublisherId(publisherId))
	}

	h := &hazelcastcomv1alpha1.Hazelcast{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: m.Spec.HazelcastResourceName, Namespace: wan.Namespace}, h); err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan,
			wanSyncFailedStatus(fmt.Errorf("failed to get Hazelcast CR for WAN Sync: %w", err)))
	}
	rest := NewRestClient(h)
	err = rest.WanSync(ctx, wan)
	if err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan,
			wanSyncFailedStatus(fmt.Errorf("failed to execute WAN Sync: %w", err)))
	}
	if err := r.Update(ctx, util.InsertLastSuccessfullyAppliedSpec(wan.Spec, wan)); err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
	}
	return updateWanSyncStatus(ctx, r.Client, wan, wanSyncSuccessStatus())
}

func (r *WanSyncReconciler) executeFinalizer(ctx context.Context, wan *hazelcastcomv1alpha1.WanSync, cli *hazelcast.Client) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(wan, n.Finalizer) {
		logger := util.GetLogger(ctx)
		logger.Info("Deleting WAN configuration")
		wpo, err := r.getWanPublisherObject(ctx, wan)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := stopWanReplication(ctx, cli, wpo); err != nil {
			return updateWanSyncStatus(ctx, r.Client, wan,
				wanSyncFailedStatus(fmt.Errorf("stopping WAN replication failed: %w", err)))
		}
		logger.Info("Deleting WAN configuration finalizer")
		controllerutil.RemoveFinalizer(wan, n.Finalizer)
		if err := r.Update(ctx, wan); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *WanSyncReconciler) getWanPublisherObject(ctx context.Context, wan *hazelcastcomv1alpha1.WanSync) (WanPublisherObject, error) {
	if wan.Spec.WanReplicationName != "" {
		wr, err := r.getWanReplicationForSync(ctx, wan)
		if err != nil {
			return nil, err
		}
		return wr, nil
	}
	return wan, nil
}

func (r *WanSyncReconciler) getWanPublisherId(
	ctx context.Context, client *hazelcast.Client, wan *hazelcastcomv1alpha1.WanSync) (string, error) {
	if wan.Spec.WanReplicationName == "" {
		return applyWanReplication(ctx, client, wan)
	}
	wr, err := r.getWanReplicationForSync(ctx, wan)
	if err != nil {
		return "", err
	}
	return wr.Status.PublisherId, nil
}

func (r *WanSyncReconciler) getWanReplicationForSync(
	ctx context.Context, wan *hazelcastcomv1alpha1.WanSync) (*hazelcastcomv1alpha1.WanReplication, error) {
	wr := &hazelcastcomv1alpha1.WanReplication{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: wan.Spec.WanReplicationName, Namespace: wan.Namespace}, wr)
	if err != nil {
		return nil, fmt.Errorf("failed to get WanReplication CR from WanSync: %w", err)
	}
	return wr, nil
}

func (r *WanSyncReconciler) getWanMap(ctx context.Context, wan *hazelcastcomv1alpha1.WanSync) (*hazelcastcomv1alpha1.Map, error) {
	var wpc *hazelcastcomv1alpha1.WanPublisherConfig
	if wan.Spec.WanReplicationName != "" {
		wr, err := r.getWanReplicationForSync(ctx, wan)
		if err != nil {
			return nil, err
		}
		wpc = &wr.Spec.WanPublisherConfig
	} else {
		wpc = wan.Spec.Config
	}
	m := &hazelcastcomv1alpha1.Map{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: wpc.MapResourceName, Namespace: wan.Namespace}, m); err != nil {
		return nil, fmt.Errorf("failed to get Map CR from WanReplication: %w", err)
	}
	return m, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastcomv1alpha1.WanSync{}).
		Complete(r)
}
