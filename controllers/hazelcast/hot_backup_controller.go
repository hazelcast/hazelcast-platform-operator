package hazelcast

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/util"
)

type HotBackupReconciler struct {
	client.Client
	Log       logr.Logger
	scheduled sync.Map
	cron      *cron.Cron
	statuses  sync.Map
}

func NewHotBackupReconciler(c client.Client, log logr.Logger) *HotBackupReconciler {
	return &HotBackupReconciler{
		Client: c,
		Log:    log,
		cron:   cron.New(),
	}
}

// Openshift related permissions
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// Role related to CRs
//+kubebuilder:rbac:groups=hazelcast.com,resources=hotbackups,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hotbackups/status,verbs=get;update;patch,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hotbackups/finalizers,verbs=update,namespace=system
// ClusterRole related to Reconcile()
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *HotBackupReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.Log.WithValues("hazelcast-hot-backup", req.NamespacedName)

	hb := &hazelcastv1alpha1.HotBackup{}
	err := r.Client.Get(ctx, req.NamespacedName, hb)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HotBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get HotBackup")
		return ctrl.Result{}, err
	}

	err = r.addFinalizer(ctx, hb, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	//Check if the HotBackup CR is marked to be deleted
	if hb.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, hb, logger)
		if err != nil {
			logger.Error(err, "Finalizer execution failed")
			return ctrl.Result{}, err
		}
		logger.V(2).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	hs, err := json.Marshal(hb.Spec)
	if err != nil {
		logger.Error(err, "Error marshaling Hot Backup as JSON")
		return reconcile.Result{}, err
	}
	if s, ok := hb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]; ok && s == string(hs) {
		logger.Info("HotBackup was already applied.", "name", hb.Name, "namespace", hb.Namespace)
		return reconcile.Result{}, nil
	}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: hb.Spec.HazelcastResourceName}, h)
	if err != nil {
		logger.Error(err, "Could not trigger Hot Backup: Hazelcast resource not found")
		return ctrl.Result{}, err
	}
	if h.Status.Phase != hazelcastv1alpha1.Running {
		err = errors.NewServiceUnavailable("Hazelcast CR is not ready")
		logger.Error(err, "Hazelcast CR is not in Running state")
		return ctrl.Result{}, err
	}
	rest := NewRestClient(h)

	if hb.Spec.Schedule != "" {
		entry, err := r.cron.AddFunc(hb.Spec.Schedule, func() {
			logger.Info("Triggering scheduled HotBackup process.", "Schedule", hb.Spec.Schedule)
			err := r.triggerHotBackup(ctx, rest, logger)
			if err != nil {
				logger.Error(err, "Hot Backups process failed")
			}
			r.reconcileHotBackupStatus(ctx, hb)
		})
		if err != nil {
			logger.Error(err, "Error creating new Schedule Hot Restart.")
		}
		logger.V(1).Info("Adding cron Job.", "EntryId", entry)
		oldV, loaded := r.scheduled.LoadOrStore(req.NamespacedName, entry)
		if loaded {
			r.cron.Remove(oldV.(cron.EntryID))
			r.scheduled.Store(req.NamespacedName, entry)
		}
		r.cron.Start()
	} else {
		err = r.triggerHotBackup(ctx, rest, logger)
		if err != nil {
			hb.Status.State = hazelcastv1alpha1.HotBackupFailure
			_ = r.Status().Update(ctx, hb)
			return ctrl.Result{}, err
		}
		r.reconcileHotBackupStatus(ctx, hb)
	}
	err = r.updateLastSuccessfulConfiguration(ctx, hb, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	return ctrl.Result{}, nil
}

func (r *HotBackupReconciler) reconcileHotBackupStatus(ctx context.Context, hb *hazelcastv1alpha1.HotBackup) {
	hzClient, ok := GetClient(types.NamespacedName{Name: hb.Spec.HazelcastResourceName, Namespace: hb.Namespace})
	if !ok {
		return
	}
	t := &StatusTicker{
		ticker: time.NewTicker(2 * time.Second),
		done:   make(chan bool),
	}
	r.statuses.Store(types.NamespacedName{Namespace: hb.Namespace, Name: hb.Name}, t)
	go func(ctx context.Context, s *StatusTicker) {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				r.updateHotBackupStatus(hzClient, ctx, hb)
			}
		}
	}(ctx, t)
}

func (r *HotBackupReconciler) updateHotBackupStatus(hzClient *HazelcastClient, ctx context.Context, h *hazelcastv1alpha1.HotBackup) {
	hb := &hazelcastv1alpha1.HotBackup{}
	namespacedName := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	err := r.Client.Get(ctx, namespacedName, hb)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("HotBackup resource not found. Ignoring since object must be deleted")
			return
		}
		r.Log.Error(err, "Failed to get HotBackup")
		return

	}
	currentState := hazelcastv1alpha1.HotBackupUnknown
	for uuid := range hzClient.Status.MemberMap {
		state := hzClient.getTimedMemberState(ctx, uuid)
		if state == nil {
			continue
		}
		r.Log.V(2).Info("Received HotBackup state for member.", "HotRestartState", state)
		currentState = hotBackupState(state.TimedMemberState.MemberState.HotRestartState, currentState)
	}
	hb.Status.State = currentState
	r.Log.V(2).Info("Updating the HotBackup status", "state", currentState)
	err = r.Status().Update(ctx, hb)
	if err != nil {
		r.Log.Error(err, "Could not update HotBackup status")
	}
	if currentState.IsFinished() {
		r.Log.Info("HotBackup task finished.", "state", currentState)
		if s, ok := r.statuses.LoadAndDelete(namespacedName); ok {
			s.(*StatusTicker).stop()
		}
	}
}

func (r *HotBackupReconciler) updateLastSuccessfulConfiguration(ctx context.Context, hb *hazelcastv1alpha1.HotBackup, logger logr.Logger) error {
	hs, err := json.Marshal(hb.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, hb, func() error {
		if hb.ObjectMeta.Annotations == nil {
			ans := map[string]string{}
			hb.ObjectMeta.Annotations = ans
		}
		hb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(hs)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Hazelcast Annotation", hb.Name, "result", opResult)
	}
	return err
}

func (r *HotBackupReconciler) addFinalizer(ctx context.Context, hb *hazelcastv1alpha1.HotBackup, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(hb, n.Finalizer) && hb.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(hb, n.Finalizer)
		err := r.Update(ctx, hb)
		if err != nil {
			return err
		}
		logger.V(1).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *HotBackupReconciler) executeFinalizer(ctx context.Context, hb *hazelcastv1alpha1.HotBackup, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(hb, n.Finalizer) {
		return nil
	}

	key := types.NamespacedName{
		Name:      hb.Name,
		Namespace: hb.Namespace,
	}
	if jobId, ok := r.scheduled.LoadAndDelete(key); ok {
		logger.V(1).Info("Removing cron Job.", "EntryId", jobId)
		r.cron.Remove(jobId.(cron.EntryID))
	}
	if s, ok := r.statuses.LoadAndDelete(key); ok {
		logger.V(1).Info("Stopping status ticker for HotBackup.", "CR", key)
		s.(*StatusTicker).stop()
	}
	controllerutil.RemoveFinalizer(hb, n.Finalizer)
	err := r.Update(ctx, hb)
	if err != nil {
		logger.Error(err, "Failed to remove finalizer from custom resource")
		return err
	}
	return nil
}

func (r *HotBackupReconciler) triggerHotBackup(ctx context.Context, rest *RestClient, logger logr.Logger) error {
	err := rest.ChangeState(ctx, Passive)
	if err != nil {
		logger.Error(err, "Error creating HotBackup. Could not change the cluster state to PASSIVE")
		return err
	}
	defer func(rest *RestClient) {
		e := rest.ChangeState(ctx, Active)
		if e != nil {
			logger.Error(e, "Could not change the cluster state to ACTIVE")
		}
	}(rest)
	err = rest.HotBackup(ctx)
	if err != nil {
		logger.Error(err, "Error creating HotBackup.")
		return err
	}
	return nil
}

func (r *HotBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.HotBackup{}).
		Complete(r)
}
