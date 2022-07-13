package hazelcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	"golang.org/x/sync/errgroup"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/upload"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type HotBackupReconciler struct {
	client.Client
	Log       logr.Logger
	scheduled sync.Map
	cron      *cron.Cron
}

func NewHotBackupReconciler(c client.Client, log logr.Logger) *HotBackupReconciler {
	return &HotBackupReconciler{
		Client: c,
		Log:    log,
		cron:   cron.New(),
	}
}

// Openshift related permissions
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=use
// Role related to CRs
//+kubebuilder:rbac:groups=hazelcast.com,resources=hotbackups,verbs=get;list;watch;create;update;patch;delete,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hotbackups/status,verbs=get;update;patch,namespace=system
//+kubebuilder:rbac:groups=hazelcast.com,resources=hotbackups/finalizers,verbs=update,namespace=system
// ClusterRole related to Reconcile()
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *HotBackupReconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	logger := r.Log.WithValues("hazelcast-hot-backup", req.NamespacedName)

	hb := &hazelcastv1alpha1.HotBackup{}
	err = r.Client.Get(ctx, req.NamespacedName, hb)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("HotBackup resource not found. Ignoring since object must be deleted")
			return result, nil
		}
		logger.Error(err, "Failed to get HotBackup")
		return r.updateStatus(ctx, req.NamespacedName, failedHbStatus(err))
	}

	err = r.addFinalizer(ctx, hb, logger)
	if err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedHbStatus(err))
	}

	//Check if the HotBackup CR is marked to be deleted
	if hb.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, hb, logger)
		if err != nil {
			return r.updateStatus(ctx, req.NamespacedName, failedHbStatus(err))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return
	}

	if hb.Status.State.IsRunning() {
		logger.Info("HotBackup is already running.",
			"name", hb.Name, "namespace", hb.Namespace, "state", hb.Status.State)
		return
	}

	if hb.Status.State.IsFinished() {
		logger.Info("HotBackup already finished.",
			"name", hb.Name, "namespace", hb.Namespace, "state", hb.Status.State)
		return
	}

	hs, err := json.Marshal(hb.Spec)
	if err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedHbStatus(fmt.Errorf("error marshaling Hot Backup as JSON: %w", err)))
	}
	if s, ok := hb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]; ok && s == string(hs) {
		logger.Info("HotBackup was already applied.", "name", hb.Name, "namespace", hb.Namespace)
		return
	}

	hazelcastName := types.NamespacedName{Namespace: req.Namespace, Name: hb.Spec.HazelcastResourceName}

	h := &hazelcastv1alpha1.Hazelcast{}
	err = r.Client.Get(ctx, hazelcastName, h)
	if err != nil {
		return r.updateStatus(ctx, req.NamespacedName, failedHbStatus(fmt.Errorf("could not trigger Hot Backup: Hazelcast resource not found: %w", err)))
	}
	if h.Status.Phase != hazelcastv1alpha1.Running {
		return r.updateStatus(ctx, req.NamespacedName, failedHbStatus(apiErrors.NewServiceUnavailable("Hazelcast CR is not ready")))
	}

	err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
		return
	}

	logger.Info("Ready to start backup")
	if hb.Spec.Schedule != "" {
		logger.Info("Adding backup to schedule")
		r.scheduleBackup(context.Background(), hb.Spec.Schedule, req.NamespacedName, hazelcastName, logger)
	} else {
		result, err = r.updateStatus(ctx, req.NamespacedName, hbWithStatus(hazelcastv1alpha1.HotBackupPending))
		if err != nil {
			return result, err
		}
		r.removeSchedule(req.NamespacedName, logger)
		go r.startBackup(context.Background(), req.NamespacedName, hazelcastName, logger) //nolint:errcheck
	}

	return
}

func (r *HotBackupReconciler) updateLastSuccessfulConfiguration(ctx context.Context, name types.NamespacedName, logger logr.Logger) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		hb := &hazelcastv1alpha1.HotBackup{}
		if err := r.Client.Get(ctx, name, hb); err != nil {
			return err
		}
		hs, err := json.Marshal(hb.Spec)
		if err != nil {
			return err
		}
		if hb.ObjectMeta.Annotations != nil {
			hb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(hs)
		}
		return r.Client.Update(ctx, hb)
	})
}

func (r *HotBackupReconciler) addFinalizer(ctx context.Context, hb *hazelcastv1alpha1.HotBackup, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(hb, n.Finalizer) && hb.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(hb, n.Finalizer)
		err := r.Update(ctx, hb)
		if err != nil {
			return err
		}
		logger.V(util.DebugLevel).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *HotBackupReconciler) executeFinalizer(ctx context.Context, hb *hazelcastv1alpha1.HotBackup, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(hb, n.Finalizer) {
		return nil
	}
	key := types.NamespacedName{
		Name:      hb.Name,
		Namespace: hb.Namespace,
	}
	r.removeSchedule(key, logger)
	controllerutil.RemoveFinalizer(hb, n.Finalizer)
	err := r.Update(ctx, hb)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *HotBackupReconciler) removeSchedule(key types.NamespacedName, logger logr.Logger) {
	if jobId, ok := r.scheduled.LoadAndDelete(key); ok {
		logger.V(util.DebugLevel).Info("Removing cron Job.", "EntryId", jobId)
		r.cron.Remove(jobId.(cron.EntryID))
	}
}

func (r *HotBackupReconciler) updateStatus(ctx context.Context, name types.NamespacedName, options hotBackupOptionsBuilder) (ctrl.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		hb := &hazelcastv1alpha1.HotBackup{}
		if err := r.Get(ctx, name, hb); err != nil {
			return err
		}
		hb.Status.State = options.status
		hb.Status.Message = options.message
		return r.Status().Update(ctx, hb)
	})

	if options.status == hazelcastv1alpha1.HotBackupFailure {
		return ctrl.Result{}, options.err
	}
	return ctrl.Result{}, err
}

func (r *HotBackupReconciler) scheduleBackup(ctx context.Context, schedule string, backupName types.NamespacedName, hazelcastName types.NamespacedName, logger logr.Logger) {
	entry, err := r.cron.AddFunc(schedule, func() {
		r.startBackup(ctx, backupName, hazelcastName, logger) //nolint:errcheck
	})
	if err != nil {
		logger.Error(err, "Error creating new Schedule Hot Restart.")
	}
	if old, loaded := r.scheduled.LoadOrStore(backupName, entry); loaded {
		r.cron.Remove(old.(cron.EntryID))
		r.scheduled.Store(backupName, entry)
	}
	r.cron.Start()
}

var (
	errBackupClientNotFound  = errors.New("client not found for hot backup CR")
	errBackupClientNoMembers = errors.New("client couldnt connect to members")
)

func (r *HotBackupReconciler) startBackup(ctx context.Context, backupName types.NamespacedName, hazelcastName types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Starting backup")
	defer logger.Info("Finished backup")

	// Change state to In Progress
	_, err := r.updateStatus(ctx, backupName, hbWithStatus(hazelcastv1alpha1.HotBackupInProgress))
	if err != nil {
		// setting status failed so this most likely will fail too
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	// Get latest version as this may be running in cron
	hz := &hazelcastv1alpha1.Hazelcast{}
	if err := r.Get(ctx, hazelcastName, hz); err != nil {
		logger.Error(err, "Get latest hazelcast CR failed")
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	logger.Info("Trigger cluster wide backup")
	if err := r.hotBackup(ctx, hz); err != nil {
		logger.Error(err, "Cluster backup failed")
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	c, ok := GetClient(hazelcastName)
	if !ok {
		return r.updateStatus(ctx, backupName, failedHbStatus(errBackupClientNotFound))
	}
	c.updateMembers(context.TODO())

	if c.Status == nil {
		return r.updateStatus(ctx, backupName, failedHbStatus(errBackupClientNoMembers))
	}

	if len(c.Status.MemberMap) == 0 {
		return r.updateStatus(ctx, backupName, failedHbStatus(errBackupClientNoMembers))
	}

	g, groupCtx := errgroup.WithContext(ctx)

	// for each member monitor and upload backup if needed
	for uuid, member := range c.Status.MemberMap {
		uuid, memberAddress := uuid, member.Address
		g.Go(func() error {
			logger := logger.WithValues("uuid", uuid)

			logger.Info("Member status monitor started")
			defer logger.Info("Member status monitor finished")

			logger.Info("Wait for member backup to finish")
			if err := waitUntilBackupSucceed(groupCtx, c, uuid); err != nil {
				return err
			}

			// skip upload for local backup
			if !hz.Spec.Persistence.IsExternal() {
				return nil
			}

			hb := &hazelcastv1alpha1.HotBackup{}
			if err := r.Get(groupCtx, backupName, hb); err != nil {
				return err
			}

			logger.Info("Start and wait for member backup upload")
			u, err := upload.NewUpload(&upload.Config{
				MemberAddress: memberAddress,
				BucketURI:     hb.Spec.BucketURI,
				BackupPath:    hz.Spec.Persistence.BaseDir,
				HazelcastName: hb.Spec.HazelcastResourceName,
				SecretName:    hb.Spec.Secret,
			})
			if err != nil {
				return err
			}

			// now start and wait for upload
			if err := u.Start(groupCtx); err != nil {
				return err
			}

			if err := u.Wait(groupCtx); err != nil {
				if errors.Is(err, context.Canceled) {
					// notify agent so we can cleanup if needed
					logger.Info("Cancel upload")
					return u.Cancel(groupCtx)
				}
				return err
			}

			// member success
			return nil
		})
	}

	logger.Info("Waiting for members")
	if err := g.Wait(); err != nil {
		logger.Error(err, "One or more members failed, returning first error")
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	logger.Info("All members finished with no errors")
	return r.updateStatus(ctx, backupName, hbWithStatus(hazelcastv1alpha1.HotBackupSuccess))
}

func (r *HotBackupReconciler) hotBackup(ctx context.Context, hz *hazelcastv1alpha1.Hazelcast) error {
	client := NewRestClient(hz)
	if err := client.ChangeState(ctx, Passive); err != nil {
		return err
	}
	defer client.ChangeState(ctx, Active) //nolint:errcheck
	return client.HotBackup(ctx)
}

func (r *HotBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.HotBackup{}).
		Complete(r)
}
