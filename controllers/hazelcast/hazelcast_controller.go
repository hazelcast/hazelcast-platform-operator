package hazelcast

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast/mutate"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// retryAfter is the time in seconds to requeue for the Pending phase
const retryAfter = 10 * time.Second

// HazelcastReconciler reconciles a Hazelcast object
type HazelcastReconciler struct {
	client.Client
	Log                   logr.Logger
	Scheme                *runtime.Scheme
	triggerReconcileChan  chan event.GenericEvent
	phoneHomeTrigger      chan struct{}
	clientRegistry        hzclient.ClientRegistry
	statusServiceRegistry hzclient.StatusServiceRegistry
}

func NewHazelcastReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cs hzclient.ClientRegistry, ssm hzclient.StatusServiceRegistry) *HazelcastReconciler {
	return &HazelcastReconciler{
		Client:                c,
		Log:                   log,
		Scheme:                s,
		triggerReconcileChan:  make(chan event.GenericEvent),
		phoneHomeTrigger:      pht,
		clientRegistry:        cs,
		statusServiceRegistry: ssm,
	}
}

// Role related to Operator UUID
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=get,namespace=system
// Openshift related permissions
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// Role related to CRs
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/status,verbs=get;update;patch,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/finalizers,verbs=update,namespace=system
// ClusterRole inherited from Hazelcast ClusterRole
//+kubebuilder:rbac:groups="",resources=endpoints;pods;nodes;services,verbs=get;list
// Role related to Reconcile()
//+kubebuilder:rbac:groups="",resources=events;services;serviceaccounts;configmaps;pods,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;watch;get;list,namespace=system
// ClusterRole related to Reconcile()
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=watch;list

func (r *HazelcastReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast", req.NamespacedName)

	h := &hazelcastv1alpha1.Hazelcast{}
	err := r.Client.Get(ctx, req.NamespacedName, h)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Hazelcast resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		return r.update(ctx, h, failedPhase(err))
	}

	// Add finalizer for Hazelcast CR to cleanup ClusterRole
	err = util.AddFinalizer(ctx, r.Client, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	// Check if the Hazelcast CR is marked to be deleted
	if h.GetDeletionTimestamp() != nil {
		// Execute finalizer's pre-delete function to cleanup ClusterRole
		r.update(ctx, h, terminatingPhase(nil)) //nolint:errcheck
		err = r.executeFinalizer(ctx, h, logger)
		if err != nil {
			return r.update(ctx, h, terminatingPhase(err).withMessage(err.Error()))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	if mutated := mutate.HazelcastSpec(h); mutated {
		err = r.Client.Update(ctx, h)
		if err != nil {
			return r.update(ctx, h,
				failedPhase(err).
					withMessage(fmt.Sprintf("error mutating new Spec: %s", err)))
		}
	}

	err = hazelcastv1alpha1.ValidateHazelcastSpec(h)
	if err != nil {
		return r.update(ctx, h,
			failedPhase(err).
				withMessage(fmt.Sprintf("error validating new Spec: %s", err)))
	}

	err = r.reconcileRole(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileClusterRole(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileServiceAccount(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileRoleBinding(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileClusterRoleBinding(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileService(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileServicePerPod(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	err = r.reconcileUnusedServicePerPod(ctx, h)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	if !r.isServicePerPodReady(ctx, h) {
		logger.Info("Service per pod is not ready, waiting.")
		return r.update(ctx, h, pendingPhase(retryAfter))
	}

	s, createdBefore := h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]

	var newExecutorServices map[string]interface{}
	if createdBefore {
		lastSpec, err := r.unmarshalHazelcastSpec(h, s)
		if err != nil {
			return r.update(ctx, h, failedPhase(err))
		}

		newExecutorServices, err = r.detectNewExecutorServices(h, lastSpec)
		if err != nil {
			return r.update(ctx, h, failedPhase(err))
		}

		err = hazelcastv1alpha1.ValidateNotUpdatableHazelcastFields(&h.Spec, lastSpec)
		if err != nil {
			return r.update(ctx, h,
				failedPhase(err).
					withMessage(fmt.Sprintf("error validating new Spec: %s", err)))
		}
	}

	err = r.reconcileConfigMap(ctx, h, logger)
	if err != nil {
		return r.update(ctx, h, failedPhase(err))
	}

	if err = r.reconcileStatefulset(ctx, h, logger); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		} else {
			return r.update(ctx, h, failedPhase(err).withMessage(err.Error()))
		}
	}

	if err = r.persistenceStartupAction(ctx, h, logger); err != nil {
		logger.V(util.WarnLevel).Info("Startup action call was unsuccessful", "error", err.Error())
		return r.update(ctx, h, pendingPhase(retryAfter))
	}

	if ok, err := util.CheckIfRunning(ctx, r.Client, req.NamespacedName, *h.Spec.ClusterSize); !ok {
		if err == nil {
			return r.update(ctx, h, pendingPhase(retryAfter))
		} else {
			return r.update(ctx, h, failedPhase(err).withMessage(err.Error()))
		}
	}

	cl, err := r.clientRegistry.Create(ctx, h)
	if err != nil {
		return r.update(ctx, h, pendingPhase(retryAfter).withMessage(err.Error()))
	}
	r.statusServiceRegistry.Create(req.NamespacedName, cl, r.Log, r.triggerReconcileChan)

	if err = r.ensureClusterActive(ctx, cl, h); err != nil {
		logger.Error(err, "Cluster activation attempt after hot restore failed")
		return r.update(ctx, h, pendingPhase(retryAfter))
	}

	if !cl.IsClientConnected() {
		r.statusServiceRegistry.Delete(req.NamespacedName)
		err = r.clientRegistry.Delete(ctx, req.NamespacedName)
		if err != nil {
			return r.update(ctx, h, pendingPhase(retryAfter).withMessage(err.Error()))
		}
		return r.update(ctx, h, pendingPhase(retryAfter).withMessage("Client is not connected to the cluster!"))
	}

	if newExecutorServices != nil {
		if !cl.AreAllMembersAccessible() {
			return r.update(ctx, h, pendingPhase(retryAfter))
		}
		r.addExecutorServices(ctx, cl, newExecutorServices)
	}

	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(h) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	err = r.updateLastSuccessfulConfiguration(ctx, h, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	externalAddrs := util.GetExternalAddresses(ctx, r.Client, h, logger)
	return r.update(ctx, h, r.runningPhaseWithStatus(req).
		withExternalAddresses(externalAddrs).
		withMessage(clientConnectionMessage(r.clientRegistry, req)))
}

func (r *HazelcastReconciler) podUpdates(pod client.Object) []reconcile.Request {
	p, ok := pod.(*corev1.Pod)
	if !ok {
		return []reconcile.Request{}
	}

	name, ok := getHazelcastCRName(p)
	if !ok {
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: p.GetNamespace(),
			},
		},
	}
}

func (r *HazelcastReconciler) mapUpdates(m client.Object) []reconcile.Request {
	mp, ok := m.(*hazelcastv1alpha1.Map)
	if !ok {
		return []reconcile.Request{}
	}

	if mp.Status.State == hazelcastv1alpha1.MapPending {
		return []reconcile.Request{}
	}
	name := mp.Spec.HazelcastResourceName

	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: mp.GetNamespace(),
			},
		},
	}
}

func getHazelcastCRName(pod *corev1.Pod) (string, bool) {
	if pod.Labels[n.ApplicationManagedByLabel] == n.OperatorName && pod.Labels[n.ApplicationNameLabel] == n.Hazelcast {
		return pod.Labels[n.ApplicationInstanceNameLabel], true
	} else {
		return "", false
	}
}

func clientConnectionMessage(cs hzclient.ClientRegistry, req ctrl.Request) string {
	c, ok := cs.Get(req.NamespacedName)
	if !ok {
		return "Operator failed to create connection to cluster, some features might be unavailable."
	}

	if !c.IsClientConnected() {
		return "Operator could not connect to the cluster. Some features might be unavailable."
	}

	if !c.AreAllMembersAccessible() {
		return "Operator could not connect to all cluster members. Some features might be unavailable."
	}

	return ""
}

func (r *HazelcastReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.CronHotBackup{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		m := rawObj.(*hazelcastv1alpha1.CronHotBackup)
		return []string{m.Spec.HotBackupTemplate.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.Map{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		m := rawObj.(*hazelcastv1alpha1.Map)
		return []string{m.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.HotBackup{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		hb := rawObj.(*hazelcastv1alpha1.HotBackup)
		return []string{hb.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.MultiMap{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		m := rawObj.(*hazelcastv1alpha1.MultiMap)
		return []string{m.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.Topic{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		t := rawObj.(*hazelcastv1alpha1.Topic)
		return []string{t.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.ReplicatedMap{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		m := rawObj.(*hazelcastv1alpha1.ReplicatedMap)
		return []string{m.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.Queue{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		m := rawObj.(*hazelcastv1alpha1.Queue)
		return []string{m.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.Cache{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		m := rawObj.(*hazelcastv1alpha1.Cache)
		return []string{m.Spec.HazelcastResourceName}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.WanReplication{}, "hazelcastResourceName", func(rawObj client.Object) []string {
		wr := rawObj.(*hazelcastv1alpha1.WanReplication)
		hzResources := []string{}
		for k := range wr.Status.WanReplicationMapsStatus {
			hzName, _ := splitWanMapKey(k)
			hzResources = append(hzResources, hzName)
		}
		return hzResources
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Hazelcast{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Watches(&source.Channel{Source: r.triggerReconcileChan}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(r.podUpdates)).
		Watches(&source.Kind{Type: &hazelcastv1alpha1.Map{}}, handler.EnqueueRequestsFromMapFunc(r.mapUpdates)).
		Complete(r)
}
