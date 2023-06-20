package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/controllers"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// JetJobSnapshotReconciler reconciles a JetJobSnapshot object
type JetJobSnapshotReconciler struct {
	client.Client
	log                  logr.Logger
	scheme               *runtime.Scheme
	clientRegistry       hzclient.ClientRegistry
	exportingSnapshotMap sync.Map
	phoneHomeTrigger     chan struct{}
	mtlsClientRegistry   mtls.HttpClientRegistry
}

func NewJetJobSnapshotReconciler(c client.Client, log logr.Logger, scheme *runtime.Scheme, cs hzclient.ClientRegistry, mtlsClientRegistry mtls.HttpClientRegistry, pht chan struct{}) *JetJobSnapshotReconciler {
	return &JetJobSnapshotReconciler{
		Client:             c,
		log:                log,
		scheme:             scheme,
		clientRegistry:     cs,
		mtlsClientRegistry: mtlsClientRegistry,
		phoneHomeTrigger:   pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobsnapshots,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobsnapshots/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobsnapshots/finalizers,verbs=update,namespace=watched

func (r *JetJobSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.log.WithValues("hazelcast-jet-job-snapshot", req.NamespacedName)

	jjs := &hazelcastv1alpha1.JetJobSnapshot{}
	err := r.Client.Get(ctx, req.NamespacedName, jjs)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("JetJobSnapshot resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get JetJobSnapshot")
		return ctrl.Result{}, fmt.Errorf("failed to get JetJobSnapshot: %w", err)
	}

	err = util.AddFinalizer(ctx, r.Client, jjs, logger)
	if err != nil {
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	//Check if the JetJobSnapshot CR is marked to be deleted
	if jjs.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, jjs, logger)
		if err != nil {
			return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
				withJetJobSnapshotFailedState(err.Error()))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	s, createdBefore := jjs.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if createdBefore {
		logger.Info("exporting JetJobSnapshot process has already been started")
		lastSpec := &hazelcastv1alpha1.JetJobSnapshotSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last JetJobSnapshot Spec: %w", err)
			return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
				withJetJobSnapshotFailedState(err.Error()))
		}
		var allErrs = hazelcastv1alpha1.ValidateJetJobSnapshotNonUpdatableFields(jjs.Spec, *lastSpec)
		if len(allErrs) > 0 {
			err = apiErrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJobSnapshot"}, req.Name, allErrs)
			return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
				withJetJobSnapshotFailedState(err.Error()))
		}
		return ctrl.Result{}, nil
	}

	jetJobNn := types.NamespacedName{
		Name:      jjs.Spec.JetJobResourceName,
		Namespace: jjs.Namespace,
	}

	jetJob := hazelcastv1alpha1.JetJob{}
	err = r.Client.Get(ctx, jetJobNn, &jetJob)
	if err != nil {
		logger.Error(err, "error on getting jet job", "jetJobResourceName", jetJobNn.Name)
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	// requeue if the jobID is not loaded yet
	if jetJob.Status.Id == 0 {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	hzNn := types.NamespacedName{
		Name:      jetJob.Spec.HazelcastResourceName,
		Namespace: jetJob.Namespace,
	}

	hz := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, hzNn, hz)
	if err != nil {
		logger.Info("Could not find hazelcast cluster", "name", hzNn, "err", err)
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	err = r.reconcileOwnerReferences(jjs, hz)
	if err != nil {
		logger.Info("Could not update owner reference", "name", jjs.Name, "err", err)
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	if err := hazelcastv1alpha1.ValidateJetJobSnapshot(hz); err != nil {
		logger.Info("License Key is not set", "name", hzNn)
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	if hz.Status.Phase != hazelcastv1alpha1.Running {
		logger.Info("Hazelcast cluster is not ready", "name", hzNn, "phase", hz.Status.Phase)
		err := fmt.Errorf("Hazelcast CR '%s' status phase is not equal to '%s'", hz.Name, hazelcastv1alpha1.Running)
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	c, err := r.clientRegistry.GetOrCreate(ctx, hzNn)
	if err != nil {
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	result, err := r.exportSnapshot(jjs, jetJob.Status.Id, c, logger)
	if err != nil {
		return result, err
	}

	err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}
	return result, nil
}

func (r *JetJobSnapshotReconciler) reconcileOwnerReferences(jjs *hazelcastv1alpha1.JetJobSnapshot, h *hazelcastv1alpha1.Hazelcast) error {
	for _, ownerRef := range jjs.OwnerReferences {
		if ownerRef.Kind == h.Kind && ownerRef.Name == h.Name && ownerRef.UID == h.UID {
			return nil
		}
	}

	return controllerutil.SetOwnerReference(h, jjs, r.Scheme())
}

func (r *JetJobSnapshotReconciler) exportSnapshot(jjs *hazelcastv1alpha1.JetJobSnapshot, jetJobId int64, c hzclient.Client, logger logr.Logger) (ctrl.Result, error) {
	if !jjs.Status.CreationTime.IsZero() {
		logger.Info("Snapshot has already been exported", "name", jjs.SnapshotName())
		return ctrl.Result{}, nil
	}

	// It guarantees that the exporting snapshot process is operated only once for each resource.
	k := types.NamespacedName{Name: jjs.Name, Namespace: jjs.Namespace}
	if _, loaded := r.exportingSnapshotMap.LoadOrStore(k, new(any)); !loaded {
		//goland:noinspection GoUnhandledErrorResult
		go func(n types.NamespacedName) {
			logger.Info("Exporting snapshot", "name", jjs.SnapshotName(), "cancelJob", jjs.Spec.CancelJob,
				"jetJobResourceName", jjs.Spec.JetJobResourceName)
			exportSnapshotCtx := context.Background()
			updateJetJobSnapshotStatusRetry(exportSnapshotCtx, r.Client, n, //nolint:errcheck
				withJetJobSnapshotState(hazelcastv1alpha1.JetJobSnapshotExporting))
			jetService := hzclient.NewJetService(c)
			t, err := jetService.ExportSnapshot(exportSnapshotCtx, jetJobId, jjs.SnapshotName(), jjs.Spec.CancelJob)
			if err != nil {
				logger.Error(err, "Error on exporting snapshot", "name", jjs.SnapshotName())
				updateJetJobSnapshotStatusRetry(exportSnapshotCtx, r.Client, n, //nolint:errcheck
					withJetJobSnapshotState(hazelcastv1alpha1.JetJobSnapshotFailed))
				return
			}
			logger.Info("Snapshot is exported successfully", "name", jjs.SnapshotName())
			setCreationTime(exportSnapshotCtx, r.Client, n, t)              //nolint:errcheck
			updateJetJobSnapshotStatusRetry(exportSnapshotCtx, r.Client, n, //nolint:errcheck
				withJetJobSnapshotState(hazelcastv1alpha1.JetJobSnapshotExported))
		}(k)
	} else {
		logger.Info("Exporting process has already been started", "name", jjs.SnapshotName(), "cancelJob", jjs.Spec.CancelJob)
	}
	return ctrl.Result{}, nil
}

func (r *JetJobSnapshotReconciler) updateLastSuccessfulConfiguration(ctx context.Context, name types.NamespacedName) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		jjs := &hazelcastv1alpha1.JetJobSnapshot{}
		if err := r.Client.Get(ctx, name, jjs); err != nil {
			return err
		}
		jjss, err := json.Marshal(jjs.Spec)
		if err != nil {
			return err
		}
		if jjs.ObjectMeta.Annotations == nil {
			jjs.ObjectMeta.Annotations = make(map[string]string)
		}
		jjs.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(jjss)

		return r.Client.Update(ctx, jjs)
	})
}

func (r *JetJobSnapshotReconciler) executeFinalizer(ctx context.Context, jjs *hazelcastv1alpha1.JetJobSnapshot, logger logr.Logger) error {
	k := types.NamespacedName{Name: jjs.Name, Namespace: jjs.Namespace}
	r.exportingSnapshotMap.Delete(k)
	if !controllerutil.ContainsFinalizer(jjs, n.Finalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(jjs, n.Finalizer)
	if err := r.Update(ctx, jjs); err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JetJobSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.JetJobSnapshot{}).
		Complete(r)
}
