package hazelcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/backup"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	localbackup "github.com/hazelcast/hazelcast-platform-operator/internal/local_backup"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/upload"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type HotBackupReconciler struct {
	client.Client
	Log                   logr.Logger
	cancelMap             map[types.NamespacedName]context.CancelFunc
	backup                map[types.NamespacedName]struct{}
	phoneHomeTrigger      chan struct{}
	mtlsClient            *mtls.Client
	clientRegistry        hzclient.ClientRegistry
	statusServiceRegistry hzclient.StatusServiceRegistry
}

func NewHotBackupReconciler(c client.Client, log logr.Logger, pht chan struct{}, mtlsClient *mtls.Client, cs hzclient.ClientRegistry, ssm hzclient.StatusServiceRegistry) *HotBackupReconciler {
	return &HotBackupReconciler{
		Client:                c,
		Log:                   log,
		cancelMap:             make(map[types.NamespacedName]context.CancelFunc),
		backup:                make(map[types.NamespacedName]struct{}),
		phoneHomeTrigger:      pht,
		mtlsClient:            mtlsClient,
		clientRegistry:        cs,
		statusServiceRegistry: ssm,
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

	err = util.AddFinalizer(ctx, r.Client, hb, logger)
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

	if hb.Status.State.IsRunning() || r.checkBackup(req.NamespacedName) {
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
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	r.cancelMap[req.NamespacedName] = cancelFunc

	result, err = r.updateStatus(ctx, req.NamespacedName, hbWithStatus(hazelcastv1alpha1.HotBackupPending))
	if err != nil {
		return result, err
	}
	r.lockBackup(req.NamespacedName)
	go func() {
		_, err := r.startBackup(cancelCtx, req.NamespacedName, hb.Spec.IsExternal(), hazelcastName, logger)
		if err != nil {
			logger.Error(err, "Error taking backup")
		}
	}()
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
		if hb.ObjectMeta.Annotations == nil {
			hb.ObjectMeta.Annotations = make(map[string]string)
		}
		hb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(hs)

		return r.Client.Update(ctx, hb)
	})
}

func (r *HotBackupReconciler) executeFinalizer(ctx context.Context, hb *hazelcastv1alpha1.HotBackup, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(hb, n.Finalizer) {
		return nil
	}
	key := types.NamespacedName{
		Name:      hb.Name,
		Namespace: hb.Namespace,
	}
	if cancelFunc, ok := r.cancelMap[key]; ok {
		cancelFunc()
		delete(r.cancelMap, key)
	}
	r.unlockBackup(key)
	controllerutil.RemoveFinalizer(hb, n.Finalizer)
	err := r.Update(ctx, hb)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *HotBackupReconciler) updateStatus(ctx context.Context, name types.NamespacedName, options *hotBackupOptionsBuilder) (ctrl.Result, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		hb := &hazelcastv1alpha1.HotBackup{}
		if err := r.Get(ctx, name, hb); err != nil {
			return err
		}
		hb.Status.State = options.status
		hb.Status.Message = options.message
		hb.Status.BackupUUIDs = options.backupUUIDs
		return r.Status().Update(ctx, hb)
	})

	if options.status == hazelcastv1alpha1.HotBackupFailure {
		return ctrl.Result{}, options.err
	}
	return ctrl.Result{}, err
}

func (r *HotBackupReconciler) checkBackup(name types.NamespacedName) bool {
	_, ok := r.backup[name]
	return ok
}

func (r *HotBackupReconciler) lockBackup(name types.NamespacedName) {
	r.backup[name] = struct{}{}
}

func (r *HotBackupReconciler) unlockBackup(name types.NamespacedName) {
	delete(r.backup, name)
}

func (r *HotBackupReconciler) startBackup(ctx context.Context, backupName types.NamespacedName, isExternal bool, hazelcastName types.NamespacedName, logger logr.Logger) (ctrl.Result, error) {
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

	client, ok := r.clientRegistry.Get(hazelcastName)
	if !ok {
		logger.Error(err, "Get Hazelcast Client failed")
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	statusService, ok := r.statusServiceRegistry.Get(hazelcastName)
	if !ok {
		logger.Error(err, "Get Hazelcast Status Service failed")
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	backupService := hzclient.NewBackupService(client)
	b, err := backup.NewClusterBackup(statusService, backupService)
	if err != nil {
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	if err := b.Start(ctx); err != nil {
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	// for each member monitor and upload backup if needed
	g, groupCtx := errgroup.WithContext(ctx)
	for _, m := range b.Members() {
		m := m
		g.Go(func() error {
			if err := m.Wait(groupCtx); err != nil {
				// cancel cluster backup
				cancelErr := b.Cancel(ctx)
				if cancelErr != nil {
					return cancelErr
				}
				return fmt.Errorf("Backup error for member %s: %w", m.UUID, err)
			}
			return nil
		})
	}

	logger.Info("Waiting for member backups to finish")
	// Wait for all local backups to finish
	if err := g.Wait(); err != nil {
		logger.Error(err, "One or more members failed, returning first error")
		return r.updateStatus(ctx, backupName, failedHbStatus(err))
	}

	backupUUIDs := make([]string, len(b.Members()))
	// for each member monitor and upload backup if needed
	g, groupCtx = errgroup.WithContext(ctx)
	for i, m := range b.Members() {
		m := m
		i := i
		g.Go(func() error {
			// if local backup
			if !isExternal {
				b, err := localbackup.NewLocalBackup(&localbackup.Config{
					MemberAddress: m.Address,
					MTLSClient:    r.mtlsClient,
					BackupBaseDir: hz.Spec.Persistence.BaseDir,
					MemberID:      i,
				})
				if err != nil {
					return err
				}
				bf, err := b.GetLatestLocalBackup(ctx)
				if err != nil {
					return err
				}
				backupUUIDs[i] = bf
				return nil
			}

			hb := &hazelcastv1alpha1.HotBackup{}
			if err := r.Get(groupCtx, backupName, hb); err != nil {
				return err
			}

			u, err := upload.NewUpload(&upload.Config{
				MemberAddress: m.Address,
				MTLSClient:    r.mtlsClient,
				BucketURI:     hb.Spec.BucketURI,
				BackupBaseDir: hz.Spec.Persistence.BaseDir,
				HazelcastName: hb.Spec.HazelcastResourceName,
				SecretName:    hb.Spec.Secret,
				MemberID:      i,
			})
			if err != nil {
				return err
			}

			// now start and wait for upload
			if err := u.Start(groupCtx); err != nil {
				return err
			}

			bk, err := u.Wait(groupCtx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					// notify agent so we can cleanup if needed
					cancelErr := u.Cancel(ctx)
					if cancelErr != nil {
						return cancelErr
					}
					return fmt.Errorf("Upload error for member %s: %w", m.UUID, err)
				}
				return err
			}

			backupUUIDs[i] = bk
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
	return r.updateStatus(ctx, backupName, hbWithStatus(hazelcastv1alpha1.HotBackupSuccess).withBackupUUIDs(backupUUIDs))
}

func (r *HotBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.HotBackup{}).
		Complete(r)
}
