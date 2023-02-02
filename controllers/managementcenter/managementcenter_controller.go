package managementcenter

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

const retryAfter = 10 * time.Second

// ManagementCenterReconciler reconciles a ManagementCenter object
type ManagementCenterReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
}

func NewManagementCenterReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}) *ManagementCenterReconciler {
	return &ManagementCenterReconciler{
		Client:           c,
		Log:              log,
		Scheme:           s,
		phoneHomeTrigger: pht,
	}
}

// Role related to CRs
//+kubebuilder:rbac:groups=hazelcast.com,resources=managementcenters,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=managementcenters/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=managementcenters/finalizers,verbs=update,namespace=watched
// Role related to Reconcile() duplicated in hazelcast_controller.go
//+kubebuilder:rbac:groups="",resources=events;services;pods,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch;create;update;patch;delete,namespace=watched
// Role related to Reconcile()
//+kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update;patch;delete,namespace=watched

func (r *ManagementCenterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("management-center", req.NamespacedName)

	mc := &hazelcastv1alpha1.ManagementCenter{}
	err := r.Client.Get(ctx, req.NamespacedName, mc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Management Center resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		return update(ctx, r.Status(), mc, failedPhase(err))
	}

	err = util.AddFinalizer(ctx, r.Client, mc, logger)
	if err != nil {
		return update(ctx, r.Client, mc, failedPhase(err))
	}

	//Check if the ManagementCenter CR is marked to be deleted
	if mc.GetDeletionTimestamp() != nil {
		// Execute finalizer's pre-delete function to delete MC metric
		err = r.executeFinalizer(ctx, mc)
		if err != nil {
			return update(ctx, r.Client, mc, failedPhase(err))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	err = r.reconcileService(ctx, mc, logger)
	if err != nil {
		return update(ctx, r.Status(), mc, failedPhase(err))
	}

	err = r.reconcileIngress(ctx, mc, logger)
	if err != nil {
		return update(ctx, r.Status(), mc, failedPhase(err))
	}

	err = r.reconcileStatefulset(ctx, mc, logger)
	if err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		} else {
			return update(ctx, r.Status(), mc, failedPhase(err))
		}
	}

	if ok, err := util.CheckIfRunning(ctx, r.Client, req.NamespacedName, 1); !ok {
		if err == nil {
			return update(ctx, r.Status(), mc, pendingPhase(retryAfter))
		} else {
			return update(ctx, r.Status(), mc, failedPhase(err).withMessage(err.Error()))
		}
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(mc) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, mc, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	externalAddrs := util.GetExternalAddresses(ctx, r.Client, mc, logger)
	return update(ctx, r.Status(), mc, runningPhase().withExternalAddresses(externalAddrs))
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagementCenterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.ManagementCenter{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func (r *ManagementCenterReconciler) updateLastSuccessfulConfiguration(ctx context.Context, h *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	hs, err := json.Marshal(h.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, h, func() error {
		if h.ObjectMeta.Annotations == nil {
			h.ObjectMeta.Annotations = map[string]string{}
		}
		h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(hs)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Management Center Annotation", h.Name, "result", opResult)
	}
	return err
}
