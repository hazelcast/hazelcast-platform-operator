package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/go-logr/logr"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// JetJobReconciler reconciles a JetJob object
type JetJobReconciler struct {
	client.Client
	Log                  logr.Logger
	ClientRegistry       hzclient.ClientRegistry
	clusterStatusChecker sync.Map
	phoneHomeTrigger     chan struct{}
}

func NewJetJobReconciler(c client.Client, log logr.Logger, cs hzclient.ClientRegistry, pht chan struct{}) *JetJobReconciler {
	return &JetJobReconciler{
		Client:           c,
		Log:              log,
		ClientRegistry:   cs,
		phoneHomeTrigger: pht,
	}
}

// Role related to CRs
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobs,verbs=get;list;watch;create;update;patch;delete,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobs/status,verbs=get;update;patch,namespace=watched
//+kubebuilder:rbac:groups=hazelcast.com,resources=jetjobs/finalizers,verbs=update,namespace=watched

func (r *JetJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result reconcile.Result, err error) {
	logger := r.Log.WithValues("hazelcast-jet-job", req.NamespacedName)

	jj := &hazelcastv1alpha1.JetJob{}
	err = r.Client.Get(ctx, req.NamespacedName, jj)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("HotBackup resource not found. Ignoring since object must be deleted")
			return result, nil
		}
		logger.Error(err, "Failed to get HotBackup")
		return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
	}

	err = util.AddFinalizer(ctx, r.Client, jj, logger)
	if err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
	}

	//Check if the HotBackup CR is marked to be deleted
	if jj.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, jj)
		if err != nil {
			return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return
	}

	if jj.Status.Phase.IsRunning() {
		logger.Info("JetJob is already running.",
			"name", jj.Name, "namespace", jj.Namespace, "state", jj.Status.Phase)
		return
	}

	if jj.Status.Phase.IsFinished() {
		r.removeJobFromChecker(jj)
		logger.Info("JetJob is already finished.",
			"name", jj.Name, "namespace", jj.Namespace, "state", jj.Status.Phase)
		return
	}

	jjs, err := json.Marshal(jj.Spec)
	if err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(fmt.Errorf("error marshaling JetJob as JSON: %w", err)))
	}
	if s, ok := jj.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]; ok && s == string(jjs) {
		logger.Info("JetJob was already applied.", "name", jj.Name, "namespace", jj.Namespace)
		return
	}

	hazelcastName := types.NamespacedName{Namespace: req.Namespace, Name: jj.Spec.HazelcastResourceName}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, hazelcastName, h)
	if err != nil {
		logger.Info("Could not find hazelcast cluster", "name", hazelcastName, "err", err)
		return r.updateStatus(ctx, req.NamespacedName, jetJobWithStatus(hazelcastv1alpha1.JetJobNotRunning))
	}

	if h.Status.Phase != hazelcastv1alpha1.Running {
		logger.Info("Hazelcast cluster is not ready", "name", hazelcastName, "phase", h.Status.Phase)
		return r.updateStatus(ctx, req.NamespacedName, jetJobWithStatus(hazelcastv1alpha1.JetJobNotRunning))
	}

	err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
		return
	}
	return r.startJetJob(ctx, jj, hazelcastName, logger)
}

func (r *JetJobReconciler) startJetJob(ctx context.Context, job *hazelcastv1alpha1.JetJob, hazelcastName types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Starting Jet Job")
	defer logger.Info("Jet Job triggered")

	jjnn := types.NamespacedName{Name: job.Name, Namespace: job.Namespace}
	c, err := r.ClientRegistry.GetOrCreate(ctx, hazelcastName)
	if err != nil {
		logger.Error(err, "Get Hazelcast Client failed")
		return r.updateStatus(ctx, jjnn, failedJetJobStatus(err))
	}
	val, _ := r.clusterStatusChecker.LoadOrStore(hazelcastName, newJetJobStatusChecker())
	checker, ok := val.(*jetJobStatusChecker)
	if !ok {
		logger.Error(fmt.Errorf("unable to cast jetJobStatusChecker"), "wrong type")
	}
	checker.storeJob(job.JobName(), jjnn)

	js := hzclient.NewJetService(c)
	metaData := codecTypes.DefaultExistingJarJobMetaData(job.JobName(), path.Join(n.UserCodeBucketPath, job.Spec.JarName))
	if job.Spec.MainClass != "" {
		metaData.MainClass = job.Spec.MainClass
	}
	if err = js.RunJob(ctx, metaData); err != nil {
		return r.updateStatus(ctx, jjnn, failedJetJobStatus(err))
	}
	checker.runChecker(ctx, js, r.updateJob, logger)
	return r.updateStatus(ctx, jjnn, jetJobWithStatus(hazelcastv1alpha1.JetJobStarting))
}

func (r *JetJobReconciler) updateJob(ctx context.Context, summary codecTypes.JobAndSqlSummary, name types.NamespacedName) {
	logger := r.Log.WithName("jet-job-status-checker").WithValues("hazelcast-jet-job", name)
	job := &hazelcastv1alpha1.JetJob{}
	err := r.Client.Get(ctx, name, job)
	if err != nil {
		logger.Error(err, "Unable to update jet job", "JetJob", name)
		return
	}

	if job.Status.Phase.IsFinished() {
		r.removeJobFromChecker(job)
	}
	job.Status.Phase = jobStatusPhase(summary.Status)
	job.Status.Id = summary.JobId
	job.Status.SubmissionTime = &metav1.Time{Time: time.Unix(0, summary.SubmissionTime*int64(time.Millisecond))}
	job.Status.CompletionTime = &metav1.Time{Time: time.Unix(0, summary.CompletionTime*int64(time.Millisecond))}
	job.Status.SuspensionCause = summary.SuspensionCause
	job.Status.FailureText = summary.FailureText
	err = r.Status().Update(ctx, job)
	if err != nil {
		logger.Error(err, "unable to update job status")
	}
}

func (r *JetJobReconciler) removeJobFromChecker(job *hazelcastv1alpha1.JetJob) {
	hzName := types.NamespacedName{Name: job.Spec.HazelcastResourceName, Namespace: job.Namespace}
	val, ok := r.clusterStatusChecker.Load(hzName)
	if !ok {
		return
	}
	checker, ok := val.(*jetJobStatusChecker)
	if !ok {
		return
	}
	checker.deleteJob(job.JobName())
}

func jobStatusPhase(readStatus int32) hazelcastv1alpha1.JetJobStatusPhase {
	switch readStatus {
	case 0:
		return hazelcastv1alpha1.JetJobNotRunning
	case 1:
		return hazelcastv1alpha1.JetJobStarting
	case 2:
		return hazelcastv1alpha1.JetJobRunning
	case 3:
		return hazelcastv1alpha1.JetJobSuspended
	case 4:
		return hazelcastv1alpha1.JetJobExportingSnapshot
	case 5:
		return hazelcastv1alpha1.JetJobCompleting
	case 6:
		return hazelcastv1alpha1.JetJobFailed
	case 7:
		return hazelcastv1alpha1.JetJobCompleted
	default:
		return hazelcastv1alpha1.JetJobNotRunning
	}
}

func (r *JetJobReconciler) updateLastSuccessfulConfiguration(ctx context.Context, name types.NamespacedName) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		jj := &hazelcastv1alpha1.JetJob{}
		if err := r.Client.Get(ctx, name, jj); err != nil {
			return err
		}
		jjs, err := json.Marshal(jj.Spec)
		if err != nil {
			return err
		}
		if jj.ObjectMeta.Annotations == nil {
			jj.ObjectMeta.Annotations = make(map[string]string)
		}
		jj.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(jjs)

		return r.Client.Update(ctx, jj)
	})
}

func (r *JetJobReconciler) executeFinalizer(ctx context.Context, jj *hazelcastv1alpha1.JetJob) error {
	if !controllerutil.ContainsFinalizer(jj, n.Finalizer) {
		return nil
	}
	r.removeJobFromChecker(jj)
	controllerutil.RemoveFinalizer(jj, n.Finalizer)
	err := r.Update(ctx, jj)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *JetJobReconciler) updateStatus(ctx context.Context, name types.NamespacedName, options *jetJobStatusBuilder) (ctrl.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		jj := &hazelcastv1alpha1.JetJob{}
		if err := r.Get(ctx, name, jj); err != nil {
			return err
		}
		jj.Status.Phase = options.status
		if options.err != nil {
			jj.Status.FailureText = options.err.Error()
		}
		return r.Status().Update(ctx, jj)
	})

	if options.status == hazelcastv1alpha1.JetJobFailed {
		return ctrl.Result{}, options.err
	}
	if options.status == hazelcastv1alpha1.JetJobNotRunning {
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Second}, nil
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *JetJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.JetJob{}).
		Complete(r)
}
