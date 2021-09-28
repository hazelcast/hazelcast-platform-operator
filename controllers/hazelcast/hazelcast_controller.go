package hazelcast

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-enterprise-operator/controllers/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// retryAfter is the time in seconds to requeue for the Pending phase
const retryAfter = 10 * time.Second

// HazelcastReconciler reconciles a Hazelcast object
type HazelcastReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Role related to CRs
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/status,verbs=get;update;patch,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/finalizers,verbs=update,namespace=system
// ClusterRole inherited from Hazelcast ClusterRole
//+kubebuilder:rbac:groups="",resources=endpoints;pods;nodes;services,verbs=get;list
// Role related to Reconcile()
//+kubebuilder:rbac:groups="",resources=events;services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch;create;update;patch;delete,namespace=system
// ClusterRole related to Reconcile()
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *HazelcastReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast", req.NamespacedName)

	h := &hazelcastv1alpha1.Hazelcast{}
	err := r.Client.Get(ctx, req.NamespacedName, h)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Hazelcast resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Hazelcast")
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	// Add finalizer for Hazelcast CR to cleanup ClusterRole
	err = r.addFinalizer(ctx, h, logger)
	if err != nil {
		logger.Error(err, "Failed to add finalizer into custom resource")
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	//Check if the Hazelcast CR is marked to be deleted
	if h.GetDeletionTimestamp() != nil {
		// Execute finalizer's pre-delete function to cleanup ClusterRole
		err = r.executeFinalizer(ctx, h, logger)
		if err != nil {
			logger.Error(err, "Finalizer execution failed")
			return update(ctx, r.Status(), h, failedPhase(err))
		}
		logger.V(1).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", finalizer)
		return ctrl.Result{}, nil
	}

	err = r.reconcileClusterRole(ctx, h, logger)
	if err != nil {
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	err = r.reconcileServiceAccount(ctx, h, logger)
	if err != nil {
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	err = r.reconcileClusterRoleBinding(ctx, h, logger)
	if err != nil {
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	err = r.reconcileService(ctx, h, logger)
	if err != nil {
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	err = r.reconcileServicePerPod(ctx, h, logger)
	if err != nil {
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	err = r.reconcileUnusedServicePerPod(ctx, h, logger)
	if err != nil {
		return update(ctx, r.Status(), h, failedPhase(err))
	}

	if !r.isServicePerPodReady(ctx, h, logger) {
		logger.Info("Service per pod is not ready, waiting.")
		return update(ctx, r.Status(), h, pendingPhase(retryAfter))
	}

	if err = r.reconcileStatefulset(ctx, h, logger); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		} else {
			return update(ctx, r.Status(), h, failedPhase(err))
		}
	}
	if !util.CheckIfRunning(ctx, r.Client, req.NamespacedName, h.Spec.ClusterSize) {
		return update(ctx, r.Status(), h, pendingPhase(retryAfter))
	}
	return update(ctx, r.Status(), h, runningPhase())
}

func (r *HazelcastReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Hazelcast{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}
