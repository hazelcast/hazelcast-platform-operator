package hazelcast

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type HotBackupReconciler struct {
	client.Client
	Log logr.Logger
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

	err = rest.ChangeState(Passive)
	if err != nil {
		logger.Error(err, "Error creating HotBackup. Could not change the cluster state to PASSIVE")
		return reconcile.Result{}, err
	}
	defer func(rest *RestClient) {
		e := rest.ChangeState(Active)
		if e != nil {
			logger.Error(e, "Could not change the cluster state to ACTIVE")
		}
	}(rest)
	err = rest.HotBackup()
	if err != nil {
		logger.Error(err, "Error creating HotBackup.")
		return reconcile.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *HotBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.HotBackup{}).
		Complete(r)
}
