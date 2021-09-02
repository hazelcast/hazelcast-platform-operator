package managementcenter

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
)

// ManagementCenterReconciler reconciles a ManagementCenter object
type ManagementCenterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hazelcast.com,resources=managementcenters,verbs=get;list;watch;create;update;patch;delete,namespace=system
// +kubebuilder:rbac:groups=hazelcast.com,resources=managementcenters/status,verbs=get;update;patch,namespace=system

func (r *ManagementCenterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("management-center", req.NamespacedName)

	mc := &hazelcastv1alpha1.ManagementCenter{}
	err := r.Client.Get(ctx, req.NamespacedName, mc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Management Center resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get ManagementCenter")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, mc, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileStatefulset(ctx, mc, logger); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementCenterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.ManagementCenter{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
