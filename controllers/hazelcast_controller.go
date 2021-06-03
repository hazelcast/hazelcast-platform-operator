package controllers

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

// HazelcastReconciler reconciles a Hazelcast object
type HazelcastReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/finalizers,verbs=update

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

	// Check if the statefulSet already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, found)
	expected, err2 := r.statefulSetForHazelcast(h)
	if err2 != nil {
		logger.Error(err, "Failed to create new StatefulSet resource")
		return ctrl.Result{}, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new StatefulSet", "StatefulSet.Namespace", expected.Namespace, "StatefulSet.Name", expected.Name)
		err = r.Create(ctx, expected)
		if err != nil {
			logger.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", expected.Namespace, "StatefulSet.Name", expected.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get StatefulSet")
		return ctrl.Result{}, err
	}

	// TODO Find a better comparison mechanism. Currently found object has so much default props.
	if !apiequality.Semantic.DeepEqual(found.Spec, expected.Spec) {
		logger.Info("Updating a StatefulSet", "StatefulSet.Namespace", expected.Namespace, "StatefulSet.Name", expected.Name)
		if err := r.Update(ctx, expected); err != nil {
			logger.Error(err, "Failed to update StatefulSet")
		}
	}

	// Check if the ServiceAccount already exists, if not create a new one
	foundServiceAccount := &corev1.ServiceAccount{}
	err = r.Get(ctx, types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, foundServiceAccount)
	expectedServiceAccount, err2 := r.serviceAccountForDiscovery(h)
	if err2 != nil {
		logger.Error(err, "Failed to create new ServiceAccount resource")
		return ctrl.Result{}, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", expectedServiceAccount.Namespace, "ServiceAccount.Name", expectedServiceAccount.Name)
		err = r.Create(ctx, expectedServiceAccount)
		if err != nil {
			logger.Error(err, "Failed to create new ServiceAccount", "ServiceAccount.Namespace", expectedServiceAccount.Namespace, "ServiceAccount.Name", expectedServiceAccount.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get ServiceAccount")
	}

	// Check if the ClusterRole already exists, if not create a new one
	foundClusterRole := &rbacv1.ClusterRole{}
	err = r.Get(ctx, types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, foundClusterRole)
	expectedClusterRole, err2 := r.clusterRoleForDiscovery(h)
	if err2 != nil {
		logger.Error(err, "Failed to create new ClusterRole resource")
		return ctrl.Result{}, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new ClusterRole", "ClusterRole.Namespace", expectedClusterRole.Namespace, "ClusterRole.Name", expectedClusterRole.Name)
		err = r.Create(ctx, expectedClusterRole)
		if err != nil {
			logger.Error(err, "Failed to create new ClusterRole", "ClusterRole.Namespace", expectedClusterRole.Namespace, "ClusterRole.Name", expectedClusterRole.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get ClusterRole")
	}

	// Check if the ClusterRoleBinding already exists, if not create a new one
	foundClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = r.Get(ctx, types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, foundClusterRoleBinding)
	expectedClusterRoleBinding, err2 := r.clusterRoleBindingForDiscovery(h)
	if err2 != nil {
		logger.Error(err, "Failed to create new ClusterRoleBinding resource")
		return ctrl.Result{}, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new ClusterRoleBinding", "ClusterRoleBinding.Namespace", expectedClusterRoleBinding.Namespace, "ClusterRoleBinding.Name", expectedClusterRoleBinding.Name)
		err = r.Create(ctx, expectedClusterRoleBinding)
		if err != nil {
			logger.Error(err, "Failed to create new ClusterRoleBinding", "ClusterRoleBinding.Namespace", expectedClusterRoleBinding.Namespace, "ClusterRoleBinding.Name", expectedClusterRoleBinding.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get ClusterRoleBinding")
	}

	return ctrl.Result{}, nil
}

func (r *HazelcastReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Hazelcast{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}

