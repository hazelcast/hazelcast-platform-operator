package hazelcast

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// WanSyncReconciler reconciles a WanSync object
type WanSyncReconciler struct {
	client.Client
	logr.Logger
	Scheme           *runtime.Scheme
	clientRegistry   hzclient.ClientRegistry
	phoneHomeTrigger chan struct{}
}

func NewWanSyncReconciler(c client.Client, log logr.Logger, scheme *runtime.Scheme, cs hzclient.ClientRegistry, pht chan struct{}) *WanSyncReconciler {
	return &WanSyncReconciler{
		Client:           c,
		Logger:           log,
		Scheme:           scheme,
		clientRegistry:   cs,
		phoneHomeTrigger: pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=wansyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=wansyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=wansyncs/finalizers,verbs=update

func (r *WanSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	wan := &hazelcastv1alpha1.WanSync{}
	if err := r.Get(ctx, req.NamespacedName, wan); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(util.DebugLevel).Info("Could not find WanSync, it is probably already deleted")
			return ctrl.Result{}, nil
		}
		return updateWanSyncStatus(ctx, r.Client, wan,
			wanSyncFailedStatus(fmt.Errorf("could not get WanSync: %w", err)))
	}

	if !controllerutil.ContainsFinalizer(wan, n.Finalizer) && wan.GetDeletionTimestamp().IsZero() {
		controllerutil.AddFinalizer(wan, n.Finalizer)
		if err := r.Update(ctx, wan); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	//Check if the WanSync CR is marked to be deleted
	if wan.GetDeletionTimestamp() != nil {
		err := r.executeFinalizer(ctx, wan, logger)
		if err != nil {
			return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	if err := hazelcastv1alpha1.ValidateWanSyncSpec(wan); err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
	}

	wr, err := r.getWanReplication(ctx, wan)
	if err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
	}
	if wr.Status.Status != hazelcastv1alpha1.WanStatusSuccess {
		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncPendingStatus())
	}

	maps, err := getMapsGroupByHazelcastName(ctx, r.Client, wr)
	if err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan,
			wanSyncFailedStatus(fmt.Errorf("could not get WanSync maps: %w", err)))
	}

	if !util.IsApplied(wan.ObjectMeta) {
		if err := r.Update(ctx, util.InsertLastAppliedSpec(wan.Spec, wan)); err != nil {
			return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
		} else {
			return updateWanSyncStatus(ctx, r.Client, wan, wanSyncStatus(maps))
		}
	}

	if err = r.runWanSyncJobs(ctx, maps, wan, wr, logger); err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(wan) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	if err = r.Update(ctx, util.InsertLastSuccessfullyAppliedSpec(wan.Spec, wan)); err != nil {
		return updateWanSyncStatus(ctx, r.Client, wan, wanSyncFailedStatus(err))
	}

	return ctrl.Result{}, nil
}

func (r *WanSyncReconciler) runWanSyncJobs(ctx context.Context, maps map[string][]hazelcastv1alpha1.Map, wan *hazelcastv1alpha1.WanSync, wr *hazelcastv1alpha1.WanReplication, logger logr.Logger) error {
	for hzResourceName, mps := range maps {
		h := &hazelcastv1alpha1.Hazelcast{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: hzResourceName, Namespace: wan.Namespace}, h); err != nil {
			return fmt.Errorf("failed to get Hazelcast CR for WAN Sync: %w", err)
		}
		hzClient, err := r.clientRegistry.GetOrCreate(ctx, types.NamespacedName{Name: hzResourceName, Namespace: wan.Namespace})
		if err != nil {
			return fmt.Errorf("failed to get Hazelcast client: %w", err)
		}
		wsrs := make([]hzclient.WanSyncMapRequest, 0, len(maps))
		for _, m := range mps {
			mapWanKey := wanMapKey(hzResourceName, m.MapName())
			if m.Status.State != hazelcastv1alpha1.MapSuccess {
				logger.Info("Not running WAN Sync for ", "mapKey", mapWanKey, "because the map state is ", m.Status.State)
				continue
			}

			if mapStatus, ok := wan.Status.WanSyncMapsStatus[mapWanKey]; ok && mapStatus.Phase != hazelcastv1alpha1.WanSyncNotStarted {
				logger.V(util.DebugLevel).Info("WAN Sync Job already queued",
					"name", m.Name, "namespace", m.Namespace, "map", m.MapName())
				continue
			}

			logger.V(util.DebugLevel).Info("Creating WanSync for map:",
				"name", m.Name, "namespace", m.Namespace)
			wsrs = append(wsrs, hzclient.NewWanSyncMapRequest(
				types.NamespacedName{Name: hzResourceName, Namespace: wan.Namespace},
				wan.Name, m.MapName(), wanName(m.MapName()), wr.PublisherId(m.Name)))
		}
		if len(wsrs) == 0 {
			return nil
		}
		wanSyncService := hzclient.NewWanSyncService(hzClient, wsrs)
		wanSyncService.StartSyncJob(ctx, r.stateEventUpdate(ctx, wan, logger), logger)
	}
	return nil
}

func (r *WanSyncReconciler) stateEventUpdate(ctx context.Context, ws *hazelcastv1alpha1.WanSync, logger logr.Logger) func(event hzclient.WanSyncMapResponse) {
	return func(event hzclient.WanSyncMapResponse) {
		logger.V(util.DebugLevel).Info(
			"Updating the WAN Sync map status", "map", event.Event.MapName(), "uuid", event.Event.UUID(), "type", event.Event.Type)
		if !event.Event.Type.IsWanSync() {
			return
		}
		ms := wanSyncMapStatus{
			phase: hazelcastv1alpha1.WanSyncPending,
		}
		if event.Event.Type.IsDone() {
			ms = wanSyncMapStatus{
				phase:       hazelcastv1alpha1.WanSyncCompleted,
				publisherId: event.Event.PublisherId(),
			}
		} else if event.Event.Type.IsError() {
			ms = wanSyncMapStatus{
				phase:       hazelcastv1alpha1.WanSyncFailed,
				message:     event.Event.Reason(),
				publisherId: event.Event.PublisherId(),
			}
		}
		key := wanMapKey(event.HazelcastName.Name, event.Event.MapName())
		logger.V(util.DebugLevel).Info("Map status to update", "key", key, "phase", ms.phase)
		err := updateWanSyncMapStatus(ctx, r.Client, types.NamespacedName{Name: ws.Name, Namespace: ws.Namespace}, key, ms)
		if err != nil {
			logger.Error(err, "unable to update map WAN Sync status")
		}
	}
}

func (r *WanSyncReconciler) getWanReplication(ctx context.Context, ws *hazelcastv1alpha1.WanSync) (*hazelcastv1alpha1.WanReplication, error) {
	var wr = &hazelcastv1alpha1.WanReplication{}
	if ws.Spec.WanReplicationResourceName != "" {
		err := r.Client.Get(ctx, types.NamespacedName{Name: ws.Spec.WanReplicationResourceName, Namespace: ws.Namespace}, wr)
		if err != nil {
			return nil, err
		}
		return wr, nil
	}
	err := r.Client.Get(ctx, types.NamespacedName{Name: ws.Name, Namespace: ws.Namespace}, wr)
	if err != nil {
		return nil, err
	}
	return wr, nil
}

func (r *WanSyncReconciler) executeFinalizer(ctx context.Context, ws *hazelcastv1alpha1.WanSync, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(ws, n.Finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(ws, n.Finalizer)
	if err := r.Update(ctx, ws); err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.WanSync{}).
		Complete(r)
}
