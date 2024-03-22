package managementcenter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
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
//+kubebuilder:rbac:groups="route.openshift.io",resources=routes,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups="route.openshift.io",resources=routes/custom-host,verbs=create,namespace=watched
//+kubebuilder:rbac:groups="route.openshift.io",resources=routes/status,verbs=get,namespace=watched

func (r *ManagementCenterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("management-center", req.NamespacedName)

	mc := &hazelcastv1alpha1.ManagementCenter{}
	err := r.Client.Get(ctx, req.NamespacedName, mc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Management Center resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	err = util.AddFinalizer(ctx, r.Client, mc, logger)
	if err != nil {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	//Check if the ManagementCenter CR is marked to be deleted
	if mc.GetDeletionTimestamp() != nil {
		// Execute finalizer's pre-delete function to delete MC metric
		err = r.executeFinalizer(ctx, mc)
		if err != nil {
			return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	if mutated := mutate(mc); mutated {
		err = r.Client.Update(ctx, mc)
		if err != nil {
			return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(fmt.Sprintf("error mutating new Spec: %s", err.Error())))
		}
	}

	err = hazelcastv1alpha1.ValidateManagementCenterSpec(mc)
	if err != nil {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	err = r.reconcileService(ctx, mc, logger)
	if err != nil {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	err = r.reconcileIngress(ctx, mc, logger)
	if err != nil {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	err = r.reconcileRoute(ctx, mc, logger)
	if err != nil {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	err = r.reconcileSecret(ctx, mc, logger)
	if err != nil {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	if reconfigured := isMCReconfigured(mc); reconfigured {
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcPhase(hazelcastv1alpha1.McPending), withConfigured(false))
	}

	err = r.reconcileStatefulset(ctx, mc, logger)
	if err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		} else {
			return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
		}
	}

	var statefulSet appsv1.StatefulSet
	if err := r.Client.Get(ctx, req.NamespacedName, &statefulSet); err != nil {
		if errors.IsNotFound(err) {
			if mc.Status.Phase == hazelcastv1alpha1.McConfiguring {
				return update(ctx, r.Client, mc, recoptions.RetryAfter(retryAfter),
					withMcPhase(hazelcastv1alpha1.McConfiguring),
				)
			}
			return update(ctx, r.Client, mc, recoptions.RetryAfter(retryAfter), withMcPhase(hazelcastv1alpha1.McPending))
		}
		return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
	}

	if ok, err := util.CheckIfRunning(ctx, r.Client, &statefulSet, 1); !ok {
		if mc.Status.Phase == hazelcastv1alpha1.McConfiguring {
			return update(ctx, r.Client, mc, recoptions.RetryAfter(retryAfter), withMcPhase(hazelcastv1alpha1.McConfiguring))
		}
		if err != nil {
			return update(ctx, r.Client, mc, recoptions.Error(err), withMcFailedPhase(err.Error()))
		}
		return update(ctx, r.Client, mc, recoptions.RetryAfter(retryAfter), withMcPhase(hazelcastv1alpha1.McPending))
	}

	if util.IsPhoneHomeEnabled() && !recoptions.IsSuccessfullyApplied(mc) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, mc, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	externalAddrs := util.GetExternalAddressesForMC(ctx, r.Client, mc, logger)
	enrichedAddrs := enrichPublicAddresses(ctx, r.Client, mc, externalAddrs)
	if mc.Spec.SecurityProviders.IsEnabled() && mc.Status.Phase == hazelcastv1alpha1.McPending {
		return update(ctx, r.Client, mc, recoptions.Empty(), withMcPhase(hazelcastv1alpha1.McConfiguring), withConfigured(true))
	}
	return update(ctx, r.Client, mc, recoptions.Empty(), withMcPhase(hazelcastv1alpha1.McRunning), withMcExternalAddresses(enrichedAddrs))
}

func mutate(mc *hazelcastv1alpha1.ManagementCenter) bool {
	oldMC := &mc
	mc.Default()
	if !reflect.DeepEqual(oldMC, &mc) {
		return true
	}
	return false
}

func isMCReconfigured(mc *hazelcastv1alpha1.ManagementCenter) bool {
	last, ok := mc.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !ok {
		return false
	}
	parsed := &hazelcastv1alpha1.ManagementCenterSpec{}
	if err := json.Unmarshal([]byte(last), parsed); err != nil {
		return false
	}
	return !reflect.DeepEqual(parsed.SecurityProviders, mc.Spec.SecurityProviders) && mc.Status.Configured
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

	opResult, err := util.Update(ctx, r.Client, h, func() error {
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

func enrichPublicAddresses(ctx context.Context, cli client.Client, mc *hazelcastv1alpha1.ManagementCenter, externalAddresses []string) []string {
	if mc.Spec.ExternalConnectivity.Ingress.IsEnabled() {
		externalAddresses = append(externalAddresses, mc.Spec.ExternalConnectivity.Ingress.Hostname+":80")
	}

	if mc.Spec.ExternalConnectivity.Route.IsEnabled() {
		route := &routev1.Route{}
		err := cli.Get(ctx, types.NamespacedName{Name: mc.Name, Namespace: mc.Namespace}, route)
		if err != nil {
			return externalAddresses
		}
		externalAddresses = append(externalAddresses, route.Spec.Host+":80")
	}

	return externalAddresses
}
