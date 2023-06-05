package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

// JetJobSnapshotReconciler reconciles a JetJobSnapshot object
type JetJobSnapshotReconciler struct {
	client.Client
	Log                  logr.Logger
	ClientRegistry       hzclient.ClientRegistry
	clusterStatusChecker sync.Map
	phoneHomeTrigger     chan struct{}
	mtlsClientRegistry   mtls.HttpClientRegistry
}

func NewJetJobSnapshotReconciler(c client.Client, log logr.Logger, cs hzclient.ClientRegistry, mtlsClientRegistry mtls.HttpClientRegistry, pht chan struct{}) *JetJobSnapshotReconciler {
	return &JetJobSnapshotReconciler{
		Client:             c,
		Log:                log,
		ClientRegistry:     cs,
		mtlsClientRegistry: mtlsClientRegistry,
		phoneHomeTrigger:   pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobsnapshots,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobsnapshots/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobsnapshots/finalizers,verbs=update,namespace=watched

func (r *JetJobSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := r.Log.WithValues("hazelcast-jet-job-snapshot", req.NamespacedName)

	jjs := &hazelcastv1alpha1.JetJobSnapshot{}
	err = r.Client.Get(ctx, req.NamespacedName, jjs)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("JetJobSnapshot resource not found. Ignoring since object must be deleted")
			return result, nil
		}
		logger.Error(err, "Failed to get JetJobSnapshot")
		// todo return
		return ctrl.Result{}, err
	}

	err = util.AddFinalizer(ctx, r.Client, jjs, logger)
	if err != nil {
		// todo return
		return ctrl.Result{}, err
	}

	//Check if the JetJobSnapshot CR is marked to be deleted
	if jjs.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, jjs, logger)
		if err != nil {
			// todo return
			return ctrl.Result{}, err
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return
	}

	_, err = json.Marshal(jjs.Spec)
	if err != nil {
		// todo return
		return ctrl.Result{}, err
	}

	// todo ? can it be triggered multiple times
	_, createdBefore := jjs.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if !createdBefore {

		fmt.Println("----------------------------------------------------")
		fmt.Println("Exporting Snapshot")
		// get jetjob
		jetJobNn := types.NamespacedName{
			Name:      jjs.Spec.JetJobResourceName,
			Namespace: jjs.Namespace,
		}
		jetJob := hazelcastv1alpha1.JetJob{}
		err := r.Client.Get(ctx, jetJobNn, &jetJob)
		if err != nil {
			return ctrl.Result{}, nil
		}

		// get hz
		hzNn := types.NamespacedName{
			Name:      jetJob.Spec.HazelcastResourceName,
			Namespace: jetJob.Namespace,
		}
		c, err := r.ClientRegistry.GetOrCreate(ctx, hzNn)
		if err != nil {
			return ctrl.Result{}, err
		}

		// cancel should be taken in cr
		req := codec.EncodeJetExportSnapshotRequest(jetJob.Status.Id, jjs.Spec.Name, false)
		_, err = c.InvokeOnRandomTarget(ctx, req, nil)
		if err != nil {
			return ctrl.Result{}, err
		}

	} else {
		fmt.Println("----------------------------------------------------")
		fmt.Println("Update reconciliation")
	}

	err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
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
