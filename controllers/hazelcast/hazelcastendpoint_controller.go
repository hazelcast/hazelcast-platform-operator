package hazelcast

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/controllers"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// HazelcastEndpointReconciler reconciles a HazelcastEndpoint object
type HazelcastEndpointReconciler struct {
	client.Client
	Log    logr.Logger
	scheme *runtime.Scheme
}

func NewHazelcastEndpointReconciler(c client.Client, log logr.Logger, s *runtime.Scheme) *HazelcastEndpointReconciler {
	return &HazelcastEndpointReconciler{
		Client: c,
		Log:    log,
		scheme: s,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcastendpoints,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcastendpoints/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcastendpoints/finalizers,verbs=update,namespace=watched
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch,namespace=watched

func (r *HazelcastEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast-endpoint", req.NamespacedName)

	logger.Info("reconciling")

	hzep := &hazelcastcomv1alpha1.HazelcastEndpoint{}
	err := r.Client.Get(ctx, req.NamespacedName, hzep)
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Info("Hazelcast resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return r.updateStatus(ctx, hzep, recoptions.Error(err), withHazelcastEndpointMessage(err.Error()))
	}

	if hzep.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	hz := &hazelcastcomv1alpha1.Hazelcast{}
	hzNn := types.NamespacedName{
		Name:      hzep.Spec.HazelcastResourceName,
		Namespace: hzep.Namespace,
	}
	err = r.Client.Get(ctx, hzNn, hz)
	if err != nil {
		logger.Info("Could not find hazelcast resource", "name", hzNn, "err", err)
		return r.updateStatus(ctx, hzep, recoptions.Error(err), withHazelcastEndpointMessage(err.Error()))
	}

	err = r.reconcileOwnerReferences(ctx, hzep, hz)
	if err != nil {
		return r.updateStatus(ctx, hzep, recoptions.Error(err), withHazelcastEndpointMessage(err.Error()))
	}

	addr, err := r.getExternalAddress(ctx, hzep)
	return r.updateStatus(ctx, hzep, recoptions.Error(err), withHazelcastEndpointAddress(addr), withHazelcastEndpointMessageByError(err))
}

func (r *HazelcastEndpointReconciler) reconcileOwnerReferences(ctx context.Context, hzep *hazelcastcomv1alpha1.HazelcastEndpoint,
	hz *hazelcastcomv1alpha1.Hazelcast) error {

	for _, ownerRef := range hzep.OwnerReferences {
		if ownerRef.Kind == hz.Kind && ownerRef.Name == hz.Name && ownerRef.UID == hz.UID {
			return nil
		}
	}

	err := controllerutil.SetOwnerReference(hz, hzep, r.Scheme())
	if err != nil {
		return err
	}

	return r.Client.Update(ctx, hzep)
}

var errorToGetExternalAddressOfHazelcastEndpointService = errors.New("cannot get the address of the hazelcast endpoint service")

func (r *HazelcastEndpointReconciler) getExternalAddress(ctx context.Context, hzep *hazelcastcomv1alpha1.HazelcastEndpoint) (string, error) {
	svc, err := getHazelcastEndpointService(ctx, r.Client, hzep)
	if err != nil {
		return "", err
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return "", errors.New("hazelcast endpoint service type is not LoadBalancer")
	}

	svcAddress := ""

	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		addr := util.GetLoadBalancerAddress(&ingress)
		if addr != "" {
			svcAddress = net.JoinHostPort(addr, strconv.Itoa(int(hzep.Spec.Port)))
			break
		}
	}

	if svcAddress == "" {
		return "", errorToGetExternalAddressOfHazelcastEndpointService
	}

	return svcAddress, nil
}

func getHazelcastEndpointService(ctx context.Context, c client.Client, hzep *hazelcastcomv1alpha1.HazelcastEndpoint) (*corev1.Service, error) {
	svcName, ok := hzep.Labels[n.HazelcastEndpointServiceLabelName]
	if !ok {
		return nil, fmt.Errorf("service label '%s' is not found in HazelcastEndpoint resource", n.HazelcastEndpointServiceLabelName)
	}
	svcNn := types.NamespacedName{
		Namespace: hzep.Namespace,
		Name:      svcName,
	}
	svc := &corev1.Service{}
	return svc, c.Get(ctx, svcNn, svc)
}

func (r *HazelcastEndpointReconciler) hzClusterServiceUpdates(service client.Object) []reconcile.Request {
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
		Watches(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(r.hzClusterServiceUpdates)).
		Complete(r)
}
