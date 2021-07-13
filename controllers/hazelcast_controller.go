package controllers

import (
	"context"

	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
//+kubebuilder:rbac:groups="",resources=events;services;rolebindings;serviceaccounts,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings;roles,verbs=get;list;watch;create;update;patch;delete,namespace=system
// ClusterRole related to Reconcile()
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// Role related to leader election
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs=get;list;watch;create;update;patch;delete,namespace=system

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
		return ctrl.Result{}, err
	}

	// Add finalizer for Hazelcast CR to cleanup ClusterRole
	err = r.addFinalizer(ctx, h, logger)
	if err != nil {
		logger.Error(err, "Failed to add finalizer into custom resource")
		return ctrl.Result{}, err
	}

	//Check if the Hazelcast CR is marked to be deleted
	if h.GetDeletionTimestamp() != nil {
		// Execute finalizer's pre-delete function to cleanup ClusterRole
		err = r.executeFinalizer(ctx, h, logger)
		if err != nil {
			logger.Error(err, "Finalizer execution failed")
			return ctrl.Result{}, err
		}
		logger.V(1).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", finalizer)
		return ctrl.Result{}, nil
	}

	err = r.reconcileClusterRole(ctx, h, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileServiceAccount(ctx, h, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileRoleBinding(ctx, h, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, h, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileStatefulset(ctx, h, logger); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			logger.V(1).Info("Statefulset resource version has been changed during create/update process.")
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *HazelcastReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Hazelcast{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
