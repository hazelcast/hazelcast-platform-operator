package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/controllers"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
	"time"
)

// JetJobSnapshotReconciler reconciles a JetJobSnapshot object
type JetJobSnapshotReconciler struct {
	client.Client
	log                  logr.Logger
	clientRegistry       hzclient.ClientRegistry
	exportingSnapshotMap sync.Map
	phoneHomeTrigger     chan struct{}
	mtlsClientRegistry   mtls.HttpClientRegistry
}

func NewJetJobSnapshotReconciler(c client.Client, log logr.Logger, cs hzclient.ClientRegistry, mtlsClientRegistry mtls.HttpClientRegistry, pht chan struct{}) *JetJobSnapshotReconciler {
	return &JetJobSnapshotReconciler{
		Client:             c,
		log:                log,
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
	} else {
		result, err := r.exportSnapshot(ctx, jjs, logger)
		if err != nil {
			return result, err
		}
	}

	err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}
	return ctrl.Result{}, nil
}

func (r *JetJobSnapshotReconciler) exportSnapshot(ctx context.Context, jjs *hazelcastv1alpha1.JetJobSnapshot, logger logr.Logger) (ctrl.Result, error) {
	// get JetJob CR
	jetJobNn := types.NamespacedName{
		Name:      jjs.Spec.JetJobResourceName,
		Namespace: jjs.Namespace,
	}

	jetJob := hazelcastv1alpha1.JetJob{}
	err := r.Client.Get(ctx, jetJobNn, &jetJob)
	if err != nil {
		logger.Error(err, "error on getting jet job", "jetJobResourceName", jetJobNn.Name)
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	// requeue, if the jobID is not loaded yet
	if jetJob.Status.Id == 0 {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// get Hazelcast CR and client
	hzNn := types.NamespacedName{
		Name:      jetJob.Spec.HazelcastResourceName,
		Namespace: jetJob.Namespace,
	}
	c, err := r.clientRegistry.GetOrCreate(ctx, hzNn)
	if err != nil {
		return updateJetJobSnapshotStatus(ctx, r.Client, jjs, recoptions.Error(err),
			withJetJobSnapshotFailedState(err.Error()))
	}

	// It guarantees that the exporting snapshot process is started only once for each resource.
	k := types.NamespacedName{Name: jjs.Name, Namespace: jetJob.Namespace}
	if _, loaded := r.exportingSnapshotMap.LoadOrStore(k, new(any)); !loaded {
		go func(n types.NamespacedName) {
			logger.Info("Exporting jet job snapshot", "name", jjs.Spec.Name, "cancelJob", jjs.Spec.CancelJob)
			exportSnapshotCtx := context.Background()
			updateJetJobSnapshotStatusRetry(exportSnapshotCtx, r.Client, n, //nolint:errcheck
				withJetJobSnapshotState(hazelcastv1alpha1.JetJobSnapshotExporting))
			jetService := hzclient.NewJetService(c)
			err := jetService.ExportSnapshot(exportSnapshotCtx, jetJob.Status.Id, jjs.Spec.Name, jjs.Spec.CancelJob)
			if err != nil {
				updateJetJobSnapshotStatusRetry(exportSnapshotCtx, r.Client, n, //nolint:errcheck
					withJetJobSnapshotState(hazelcastv1alpha1.JetJobSnapshotFailed))
				return
			}
			updateJetJobSnapshotStatusRetry(exportSnapshotCtx, r.Client, n, //nolint:errcheck
				withJetJobSnapshotState(hazelcastv1alpha1.JetJobSnapshotExported))
		}(k)
	} else {
		logger.Info("Exporting process has already been started", "name", jjs.Spec.Name, "cancelJob", jjs.Spec.CancelJob)
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
