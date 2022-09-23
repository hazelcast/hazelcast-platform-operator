package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	"github.com/robfig/cron/v3"
)

// CronHotBackupReconciler reconciles a CronHotBackup object
type CronHotBackupReconciler struct {
	client.Client
	Log              logr.Logger
	scheduled        sync.Map
	cronParser       cron.Parser
	cron             *cron.Cron
	Scheme           *runtime.Scheme
	phoneHomeTrigger chan struct{}
}

func NewCronHotBackupReconciler(
	client client.Client, log logr.Logger, scheme *runtime.Scheme, pht chan struct{}) *CronHotBackupReconciler {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return &CronHotBackupReconciler{
		Client:           client,
		Log:              log,
		cronParser:       parser,
		cron:             cron.New(cron.WithParser(parser)),
		Scheme:           scheme,
		phoneHomeTrigger: pht,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=cronhotbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=cronhotbackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=cronhotbackups/finalizers,verbs=update

func (r *CronHotBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := r.Log.WithValues("hazelcast-hot-backup", req.NamespacedName)

	chb := &hazelcastv1alpha1.CronHotBackup{}
	err = r.Client.Get(ctx, req.NamespacedName, chb)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			logger.Info("HotBackup resource not found. Ignoring since object must be deleted")
			return
		}
		logger.Error(err, "Failed to get CronHotBackup")
		return
	}

	err = r.addFinalizer(ctx, chb)
	if err != nil {
		return
	}

	// Check if the CronHotBackup CR is marked to be deleted
	if chb.GetDeletionTimestamp() != nil {
		err = r.executeFinalizer(ctx, chb)
		if err != nil {
			return
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return
	}

	err = r.cleanupResources(ctx, *chb)
	if err != nil {
		logger.Error(err, "Error cleaning up HotBackup resources")
	}

	// If the CronHotBackup is not successfully applied yet
	if !util.IsSuccessfullyApplied(chb) {
		err = r.updateSchedule(ctx, chb)
		if err != nil {
			return
		}
		if util.IsPhoneHomeEnabled() {
			go func() { r.phoneHomeTrigger <- struct{}{} }()
		}
		err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
		return
	}

	// If operator does not have a cron job for the CronHotBackup resource, might be caused by a restart
	if _, ok := r.scheduled.Load(req.NamespacedName); !ok {
		err = r.updateSchedule(ctx, chb)
		if err != nil {
			return
		}
		err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
		return
	}

	applied, err := r.isAlreadyApplied(chb)
	if err != nil {
		return
	}
	// If CronHotBackup spec is updated
	if !applied {
		err = r.updateSchedule(ctx, chb)
		if err != nil {
			return
		}
		err = r.updateLastSuccessfulConfiguration(ctx, req.NamespacedName)
		return
	}

	return
}

func (r *CronHotBackupReconciler) addFinalizer(ctx context.Context, chb *hazelcastv1alpha1.CronHotBackup) error {
	if !controllerutil.ContainsFinalizer(chb, n.Finalizer) && chb.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(chb, n.Finalizer)
		err := r.Update(ctx, chb)
		if err != nil {
			return err
		}
		r.Log.V(util.DebugLevel).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *CronHotBackupReconciler) executeFinalizer(ctx context.Context, chb *hazelcastv1alpha1.CronHotBackup) error {
	if !controllerutil.ContainsFinalizer(chb, n.Finalizer) {
		return nil
	}
	r.removeSchedule(types.NamespacedName{Name: chb.Name, Namespace: chb.Namespace})
	err := r.deleteDependentHotBackups(ctx, chb)
	if err != nil {
		return fmt.Errorf("failed to delete dependent HotBackups: %w", err)
	}
	controllerutil.RemoveFinalizer(chb, n.Finalizer)
	err = r.Update(ctx, chb)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *CronHotBackupReconciler) removeSchedule(key types.NamespacedName) {
	if jobId, ok := r.scheduled.LoadAndDelete(key); ok {
		r.Log.V(util.DebugLevel).Info("Removing cron Job.", "EntryId", jobId)
		r.cron.Remove(jobId.(cron.EntryID))
	}
}

func (r *CronHotBackupReconciler) deleteDependentHotBackups(ctx context.Context, chb *hazelcastv1alpha1.CronHotBackup) error {
	fieldMatcher := client.MatchingFields{"controller": chb.Name}
	nsMatcher := client.InNamespace(chb.Namespace)

	hbl := &hazelcastv1alpha1.HotBackupList{}

	if err := r.Client.List(ctx, hbl, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("Could not get CronHotBackup dependent HotBackup resources %w", err)
	}

	if len(hbl.Items) == 0 {
		return nil
	}

	g, groupCtx := errgroup.WithContext(ctx)
	for i := 0; i < len(hbl.Items); i++ {
		i := i
		g.Go(func() error {
			return util.DeleteObject(groupCtx, r.Client, &hbl.Items[i])
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("Error deleting HotBackup resources %w", err)
	}

	if err := r.Client.List(ctx, hbl, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("CronHotBackup dependent HotBackup resources are not deleted yet %w", err)
	}

	if len(hbl.Items) != 0 {
		r.Log.Info("Items are", "items", hbl.Items)
		return fmt.Errorf("CronHotBackup dependent HotBackup resources are not deleted yet.")
	}

	return nil
}

func (r *CronHotBackupReconciler) updateSchedule(ctx context.Context, chb *hazelcastv1alpha1.CronHotBackup) error {
	chbCopy := *chb.DeepCopy()
	chbKey := types.NamespacedName{Name: chbCopy.Name, Namespace: chbCopy.Namespace}

	if chb.Spec.Suspend {
		if val, loaded := r.scheduled.LoadAndDelete(chbKey); loaded {
			r.cron.Remove(val.(cron.EntryID))
		}
		return nil
	}

	entry, err := r.cron.AddFunc(chb.Spec.Schedule, func() {
		cerr := r.createHotBackup(ctx, chbCopy)
		if cerr != nil {
			r.Log.Error(cerr, "Error creating HotBackup")
		}
	})
	if err != nil {
		return err
	}
	if old, loaded := r.scheduled.LoadOrStore(chbKey, entry); loaded {
		r.cron.Remove(old.(cron.EntryID))
		r.scheduled.Store(chbKey, entry)
	}
	r.cron.Start()
	return nil
}

func (r *CronHotBackupReconciler) createHotBackup(ctx context.Context, chb hazelcastv1alpha1.CronHotBackup) error {
	hbName, err := r.generateBackupName(chb)
	if err != nil {
		return err
	}

	hb := &hazelcastv1alpha1.HotBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        hbName,
			Namespace:   chb.Namespace,
			Annotations: chb.Spec.HotBackupTemplate.Annotations,
			Labels:      chb.Spec.HotBackupTemplate.Labels,
		},
		Spec: chb.Spec.HotBackupTemplate.Spec,
	}

	err = controllerutil.SetControllerReference(&chb, hb, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on HotBackup: %w", err)
	}

	err = r.Client.Create(ctx, hb)
	return err
}
func (r *CronHotBackupReconciler) generateBackupName(chb hazelcastv1alpha1.CronHotBackup) (string, error) {
	schedule, err := r.cronParser.Parse(chb.Spec.Schedule)
	if err != nil {
		return "", err
	}

	nextTimeInUnix := schedule.Next(time.Now()).Unix()
	return fmt.Sprintf("%s-%d", chb.Name, nextTimeInUnix), nil
}

func (r *CronHotBackupReconciler) cleanupResources(ctx context.Context, chb hazelcastv1alpha1.CronHotBackup) error {
	fieldMatcher := client.MatchingFields{"controller": chb.Name}
	nsMatcher := client.InNamespace(chb.Namespace)

	hbl := &hazelcastv1alpha1.HotBackupList{}
	if err := r.Client.List(ctx, hbl, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("Could not get CronHotBackup dependent HotBackup resources %w", err)
	}

	if len(hbl.Items) == 0 {
		return nil
	}

	failedHotBackups := []hazelcastv1alpha1.HotBackup{}
	successfulHotBackups := []hazelcastv1alpha1.HotBackup{}

	for _, hb := range hbl.Items {
		switch hb.Status.State {
		case hazelcastv1alpha1.HotBackupFailure:
			failedHotBackups = append(failedHotBackups, hb)
		case hazelcastv1alpha1.HotBackupSuccess:
			successfulHotBackups = append(successfulHotBackups, hb)
		}
	}

	deleteHotBackupHistory(ctx, r.Client, int(*chb.Spec.FailedHotBackupsHistoryLimit), failedHotBackups)
	deleteHotBackupHistory(ctx, r.Client, int(*chb.Spec.SuccessfulHotBackupsHistoryLimit), successfulHotBackups)

	return nil
}

// Best effort deletion of the HotBackup resources
func deleteHotBackupHistory(ctx context.Context, cl client.Client, limit int, backups []hazelcastv1alpha1.HotBackup) {
	diff := len(backups) - limit
	if diff <= 0 {
		return
	}

	sort.Slice(backups, func(a, b int) bool {
		return backups[a].CreationTimestamp.Before(&backups[b].CreationTimestamp)
	})
	for _, hb := range backups[:diff] {
		util.DeleteObject(ctx, cl, &hb) //nolint:errcheck
	}
}

func (r *CronHotBackupReconciler) updateLastSuccessfulConfiguration(ctx context.Context, name types.NamespacedName) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Always fetch the new version of the resource
		chb := &hazelcastv1alpha1.CronHotBackup{}
		if err := r.Client.Get(ctx, name, chb); err != nil {
			return err
		}
		hs, err := json.Marshal(chb.Spec)
		if err != nil {
			return err
		}
		if chb.ObjectMeta.Annotations == nil {
			chb.ObjectMeta.Annotations = make(map[string]string)
		}
		chb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(hs)

		return r.Client.Update(ctx, chb)
	})
}

func (r *CronHotBackupReconciler) isAlreadyApplied(chb *hazelcastv1alpha1.CronHotBackup) (bool, error) {
	chbs, err := json.Marshal(chb.Spec)
	if err != nil {
		return false, fmt.Errorf("Error marshaling the CronHotbackupSpec")
	}

	if s, ok := chb.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation]; ok && s == string(chbs) {
		r.Log.Info("CronHotBackup was already applied.", "name", chb.Name, "namespace", chb.Namespace)
		return true, nil
	}

	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronHotBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &hazelcastv1alpha1.HotBackup{}, "controller", func(rawObj client.Object) []string {
		hb := rawObj.(*hazelcastv1alpha1.HotBackup)
		controller := metav1.GetControllerOf(hb)
		if controller == nil {
			return nil
		}
		if controller.APIVersion != hazelcastv1alpha1.GroupVersion.String() || controller.Kind != "CronHotBackup" {
			return nil
		}
		return []string{controller.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.CronHotBackup{}).
		Owns(&hazelcastv1alpha1.HotBackup{}).
		Complete(r)
}
