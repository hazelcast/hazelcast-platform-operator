package hazelcast

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	kerrors "k8s.io/apimachinery/pkg/api/errors"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			logger.V(util.DebugLevel).Info("Could not find WanReplication, it is probably already deleted")
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	ctx = context.WithValue(ctx, LogKey("logger"), logger)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WanSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastcomv1alpha1.WanSync{}).
		Complete(r)
}
