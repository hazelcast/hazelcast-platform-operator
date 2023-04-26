package hazelcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/hazelcast/platform-operator-agent/sidecar"
	"golang.org/x/sync/errgroup"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// JetJobReconciler reconciles a JetJob object
type JetJobReconciler struct {
	client.Client
	Log                  logr.Logger
	ClientRegistry       hzclient.ClientRegistry
	clusterStatusChecker sync.Map
	phoneHomeTrigger     chan struct{}
	mtlsClientRegistry   mtls.HttpClientRegistry
}

func NewJetJobReconciler(c client.Client, log logr.Logger, cs hzclient.ClientRegistry, mtlsClientRegistry mtls.HttpClientRegistry, pht chan struct{}) *JetJobReconciler {
	return &JetJobReconciler{
		Client:             c,
		Log:                log,
		ClientRegistry:     cs,
		mtlsClientRegistry: mtlsClientRegistry,
		phoneHomeTrigger:   pht,
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
			logger.Info("JetJob resource not found. Ignoring since object must be deleted")
			return result, nil
		}
		logger.Error(err, "Failed to get JetJob")
		return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
	}

	err = util.AddFinalizer(ctx, r.Client, jj, logger)
	if err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
	}

	//Check if the JetJob CR is marked to be deleted
	if jj.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, jj, logger)
		if err != nil {
			return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return
	}

	if jj.Status.Phase.IsRunning() && jj.Spec.State == hazelcastv1alpha1.RunningJobState {
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
	s, createdBefore := jj.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]
	if createdBefore {
		if s == string(jjs) {
			logger.Info("JetJob was already applied.", "name", jj.Name, "namespace", jj.Namespace)
			return
		}

		lastSpec := &hazelcastv1alpha1.JetJobSpec{}
		err = json.Unmarshal([]byte(s), lastSpec)
		if err != nil {
			err = fmt.Errorf("error unmarshaling Last JetJob Spec: %w", err)
			return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
		}
		var allErrs = hazelcastv1alpha1.ValidateJetJobNonUpdatableFields(jj.Spec, *lastSpec)
		if len(allErrs) > 0 {
			err = apiErrors.NewInvalid(schema.GroupKind{Group: "hazelcast.com", Kind: "JetJob"}, req.Name, allErrs)
			return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
		}
	} else {
		jjList := &hazelcastv1alpha1.JetJobList{}
		nsMatcher := client.InNamespace(req.Namespace)
		if err = r.Client.List(ctx, jjList, nsMatcher); err != nil {
			logger.Error(err, "Unable to fetch JetJobList")
		}
		if err = hazelcastv1alpha1.ValidateExistingJobName(jj, jjList); err != nil {
			return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
		}
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

	if err = hazelcastv1alpha1.ValidateJetConfiguration(h); err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedJetJobStatus(err))
	}
	result, err = r.applyJetJob(ctx, jj, hazelcastName, logger)
	if err != nil {
		return result, err
	}

	err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}
	return
}

func (r *JetJobReconciler) applyJetJob(ctx context.Context, job *hazelcastv1alpha1.JetJob, hazelcastName types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Applying Jet Job")
	defer logger.Info("Jet Job applied")

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
	js := hzclient.NewJetService(c)
	if job.Spec.State != hazelcastv1alpha1.RunningJobState {
		err = r.changeJobState(ctx, job, js)
		return ctrl.Result{}, err
	}
	if job.Status.Phase.IsSuspended() && job.Spec.State == hazelcastv1alpha1.RunningJobState {
		err = js.ResumeJob(ctx, job.Status.Id)
		return ctrl.Result{}, err
	}

	checker.storeJob(job.JobName(), jjnn)

	metaData := codecTypes.DefaultExistingJarJobMetaData(job.JobName(), path.Join(n.JetJobJarsBucketPath, job.Spec.JarName))
	if job.Spec.MainClass != "" {
		metaData.MainClass = job.Spec.MainClass
	}
	go func(ctx context.Context, metaData codecTypes.JobMetaData, jjnn types.NamespacedName, client hzclient.Client, logger logr.Logger) {
		if job.Spec.BucketConfiguration != nil {
			logger.V(util.DebugLevel).Info("Downloading the JAR file before running the JetJob", "jj", jjnn)
			if err = r.downloadFile(ctx, job, hazelcastName, jjnn, client, logger); err != nil {
				logger.Error(err, "Error downloading Jar for JetJob")
				_, _ = r.updateStatus(ctx, jjnn, failedJetJobStatus(err))
				return
			}
			logger.V(util.DebugLevel).Info("JAR downloaded, starting the JetJob", "jj", jjnn)
		}
		if err = js.RunJob(ctx, metaData); err != nil {
			logger.Error(err, "Error running JetJob")
			_, _ = r.updateStatus(ctx, jjnn, failedJetJobStatus(err))
		}
	}(ctx, metaData, jjnn, c, logger)
	checker.runChecker(ctx, js, r.updateJob, logger)
	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(job) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}
	return r.updateStatus(ctx, jjnn, jetJobWithStatus(hazelcastv1alpha1.JetJobStarting))
}

func (r *JetJobReconciler) downloadFile(ctx context.Context, job *hazelcastv1alpha1.JetJob, hazelcastName types.NamespacedName, jjnn types.NamespacedName, client hzclient.Client, logger logr.Logger) error {
	bc := job.Spec.BucketConfiguration
	g, groupCtx := errgroup.WithContext(ctx)
	mtlsClient, ok := r.mtlsClientRegistry.Get(ctx, hazelcastName)
	if !ok {
		returnErr := errors.New("failed to get MTLS client")
		_, _ = r.updateStatus(ctx, jjnn, failedJetJobStatus(returnErr))
	}
	for _, m := range client.OrderedMembers() {
		m := m
		g.Go(func() error {
			host, _, err := net.SplitHostPort(m.Address.String())
			if err != nil {
				return err
			}
			fds, err := rest.NewFileDownloadService("https://"+host+":8443", mtlsClient)
			if err != nil {
				logger.Error(err, "unable to create NewFileDownloadService")
				return err
			}
			_, err = fds.Download(groupCtx, sidecar.DownloadFileReq{
				URL:        bc.BucketURI,
				FileName:   job.Spec.JarName,
				DestDir:    n.JetJobJarsBucketPath,
				SecretName: bc.Secret,
			})
			if err != nil {
				logger.Error(err, "unable to download Jar file")
			}
			return err
		})
	}
	return g.Wait()
}

func (r *JetJobReconciler) changeJobState(ctx context.Context, jj *hazelcastv1alpha1.JetJob, js hzclient.JetService) error {
	jtj := codecTypes.JetTerminateJob{
		JobId:         jj.Status.Id,
		TerminateMode: jetJobTerminateMode(jj.Spec.State),
	}
	return js.UpdateJobState(ctx, jtj)
}

func jetJobTerminateMode(jjs hazelcastv1alpha1.JetJobState) codecTypes.TerminateMode {
	switch jjs {
	case hazelcastv1alpha1.SuspendedJobState:
		return codecTypes.SuspendGracefully
	case hazelcastv1alpha1.CanceledJobState:
		return codecTypes.CancelGracefully
	case hazelcastv1alpha1.RestartedJobState:
		return codecTypes.RestartGracefully
	default:
		return codecTypes.CancelGracefully
	}
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
	if summary.CompletionTime != 0 {
		job.Status.CompletionTime = &metav1.Time{Time: time.Unix(0, summary.CompletionTime*int64(time.Millisecond))}
	}
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

func (r *JetJobReconciler) executeFinalizer(ctx context.Context, jj *hazelcastv1alpha1.JetJob, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(jj, n.Finalizer) {
		return nil
	}
	if err := r.stopJetExecution(ctx, jj, logger); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}
	r.removeJobFromChecker(jj)
	controllerutil.RemoveFinalizer(jj, n.Finalizer)
	if err := r.Update(ctx, jj); err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *JetJobReconciler) stopJetExecution(ctx context.Context, jj *hazelcastv1alpha1.JetJob, logger logr.Logger) error {
	if jj.Status.Id == 0 {
		logger.Info("Jet job ID is 0", "name", jj.Name, "namespace", jj.Namespace)
		return nil
	}
	hzNn := types.NamespacedName{Name: jj.Spec.HazelcastResourceName, Namespace: jj.Namespace}
	hz := &hazelcastv1alpha1.Hazelcast{}
	err := r.Client.Get(ctx, hzNn, hz)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("Hazelcast resource not found. Ignoring since object must be deleted")
			return nil
		} else {
			return err
		}
	}
	c, err := r.ClientRegistry.GetOrCreate(ctx, hzNn)
	if err != nil {
		logger.Error(err, "Get Hazelcast Client failed")
		return err
	}
	js := hzclient.NewJetService(c)
	terminateJob := codecTypes.JetTerminateJob{
		JobId:         jj.Status.Id,
		TerminateMode: codecTypes.CancelForcefully,
	}
	if err = js.UpdateJobState(ctx, terminateJob); err != nil {
		logger.Error(err, "Could not terminate JetJob")
		return err
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
