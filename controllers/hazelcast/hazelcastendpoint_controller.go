package hazelcast

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

// HazelcastEndpointReconciler reconciles a HazelcastEndpoint object
type HazelcastEndpointReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func NewHazelcastEndpointReconciler(c client.Client, log logr.Logger, s *runtime.Scheme) *HazelcastEndpointReconciler {
	return &HazelcastEndpointReconciler{
		Client: c,
		Log:    log,
		Scheme: s,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcastendpoints,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcastendpoints/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcastendpoints/finalizers,verbs=update,namespace=watched
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch,namespace=watched

func (r *HazelcastEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-endpoint", req.NamespacedName)

	logger.Info("reconciling", "request", req)
	/*
		hzEndpoint := hazelcastcomv1alpha1.HazelcastEndpoint{}
		err := r.Client.Get(ctx, req.NamespacedName, &hzEndpoint)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Hazelcast resource not found. Ignoring since object must be deleted")
				return ctrl.Result{}, nil
			}
			//return r.update(ctx, h, recoptions.Error(err), withHzFailedPhase(err.Error()))
		}
	*/

	//TODO reconcile
	//TODO validation

	return ctrl.Result{}, nil
}

func (r *HazelcastEndpointReconciler) serviceUpdates(service client.Object) []reconcile.Request {
	svc, ok := service.(*corev1.Service)
	if !ok {
		return []reconcile.Request{}
	}

	var reqs []reconcile.Request

	if isHazelcastService(svc) {
		hzEndpoints, err := listHazelcastEndpointsReferringToService(context.Background(), svc, r.Client)
		if err != nil {
			logger := r.Log.WithValues("hazelcast-endpoint-service",
				types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name})
			logger.Error(err, "failed on list HazelcastEndpoints referring to the service", "service", svc.Name)
		}
		for _, hzEndpoint := range hzEndpoints {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: hzEndpoint.Namespace,
					Name:      hzEndpoint.Name,
				},
			}
			reqs = append(reqs, req)
		}
	}

	return reqs
}

func isHazelcastService(svc *corev1.Service) bool {
	managedBy, ok := svc.Labels[n.ApplicationManagedByLabel]
	if !ok || managedBy != n.OperatorName {
		return false
	}
	appName, ok := svc.Labels[n.ApplicationNameLabel]
	if !ok || appName != n.Hazelcast {
		return false
	}
	return true
}

func listHazelcastEndpointsReferringToService(ctx context.Context, svc *corev1.Service, c client.Client) ([]hazelcastcomv1alpha1.HazelcastEndpoint, error) {
	hzEndpointList := hazelcastcomv1alpha1.HazelcastEndpointList{}

	lbl := map[string]string{
		n.HazelcastEndpointServiceLabelName: svc.Name,
	}
	nsOpt := client.InNamespace(svc.Namespace)
	lblOpt := client.MatchingLabels(lbl)
	err := c.List(ctx, &hzEndpointList, nsOpt, lblOpt)
	if err != nil {
		return nil, err
	}
	return hzEndpointList.Items, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HazelcastEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastcomv1alpha1.HazelcastEndpoint{}).
		Watches(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(r.serviceUpdates)).
		Complete(r)
}
