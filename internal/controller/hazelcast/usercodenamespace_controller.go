package hazelcast

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"

	"github.com/go-logr/logr"
	"golang.org/x/sync/errgroup"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/rest"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// UserCodeNamespaceReconciler reconciles a UserCodeNamespace object
type UserCodeNamespaceReconciler struct {
	client.Client
	logr.Logger
	Scheme             *runtime.Scheme
	clientRegistry     hzclient.ClientRegistry
	phoneHomeTrigger   chan struct{}
	mtlsClientRegistry mtls.HttpClientRegistry
}

func NewUserCodeNamespaceReconciler(c client.Client, log logr.Logger, s *runtime.Scheme, pht chan struct{}, cr *hzclient.HazelcastClientRegistry, mcr mtls.HttpClientRegistry) *UserCodeNamespaceReconciler {
	return &UserCodeNamespaceReconciler{
		Client:             c,
		Logger:             log,
		Scheme:             s,
		clientRegistry:     cr,
		phoneHomeTrigger:   pht,
		mtlsClientRegistry: mcr,
	}
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=usercodenamespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=usercodenamespaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=usercodenamespaces/finalizers,verbs=update

func (r *UserCodeNamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.WithValues("name", req.Name, "namespace", req.NamespacedName)

	ucn := &hazelcastv1alpha1.UserCodeNamespace{}
	if err := r.Get(ctx, req.NamespacedName, ucn); err != nil {
		if kerrors.IsNotFound(err) {
			logger.V(util.DebugLevel).Info("Could not find UserCodeNamespace, it is probably already deleted")
			return ctrl.Result{}, nil
		}
		return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn,
			userCodeNamepsaceFailedStatus(fmt.Errorf("could not get UserCodeNamespace: %w", err)))
	}

	if !controllerutil.ContainsFinalizer(ucn, n.Finalizer) && ucn.GetDeletionTimestamp().IsZero() {
		controllerutil.AddFinalizer(ucn, n.Finalizer)
		if err := r.Update(ctx, ucn); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if ucn.GetDeletionTimestamp() != nil {
		err := r.executeFinalizer(ctx, ucn, logger)
		if err != nil {
			return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsaceFailedStatus(err))
		}
		logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
		return ctrl.Result{}, nil
	}

	if !controller.IsApplied(ucn.ObjectMeta) {
		if err := r.Update(ctx, controller.InsertLastAppliedSpec(ucn.Spec, ucn)); err != nil {
			return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsaceFailedStatus(err))
		} else {
			return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsacePendingStatus())
		}
	}

	c, err := r.clientRegistry.GetOrCreate(ctx, types.NamespacedName{
		Namespace: ucn.Namespace,
		Name:      ucn.Spec.HazelcastResourceName,
	})
	if err != nil {
		logger.Error(err, "Get Hazelcast Client failed")
		return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsaceFailedStatus(err))
	}

	if err := r.downloadBundle(ctx, ucn, c, logger); err != nil {
		logger.Error(err, "Error downloading Jar for UserCodeNamespace")
		return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsaceFailedStatus(err))
	}

	if err := r.applyConfig(ctx, ucn, c, logger); err != nil {
		logger.Error(err, "Error applying dynamic config for UserCodeNamespace")
		return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsaceFailedStatus(err))
	}

	if util.IsPhoneHomeEnabled() && !controller.IsSuccessfullyApplied(ucn) {
		go func() { r.phoneHomeTrigger <- struct{}{} }()
	}

	if err := r.Update(ctx, controller.InsertLastSuccessfullyAppliedSpec(ucn.Spec, ucn)); err != nil {
		return updateUserCodeNamepsaceStatus(ctx, r.Client, ucn, userCodeNamepsaceFailedStatus(err))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserCodeNamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.UserCodeNamespace{}).
		Complete(r)
}

func (r *UserCodeNamespaceReconciler) executeFinalizer(ctx context.Context, ucn *hazelcastv1alpha1.UserCodeNamespace, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(ucn, n.Finalizer) {
		return nil
	}

	controllerutil.RemoveFinalizer(ucn, n.Finalizer)
	if err := r.Update(ctx, ucn); err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *UserCodeNamespaceReconciler) downloadBundle(ctx context.Context, ucn *hazelcastv1alpha1.UserCodeNamespace, client hzclient.Client, logger logr.Logger) error {
	logger.Info("Downloading UserCodeNamespace bundle")
	g, groupCtx := errgroup.WithContext(ctx)
	mtlsClient, ok := r.mtlsClientRegistry.Get(ucn.Namespace)
	if !ok {
		return errors.New("failed to get MTLS client")
	}
	for _, m := range client.OrderedMembers() {
		m := m
		g.Go(func() error {
			host, _, err := net.SplitHostPort(m.Address.String())
			if err != nil {
				return err
			}
			bs, err := rest.NewBundleService("https://"+host+":8443", mtlsClient)
			if err != nil {
				logger.Error(err, "unable to create BundleService")
				return err
			}
			_, err = bs.Download(groupCtx, rest.BundleReq{
				SecretName: ucn.Spec.BucketConfiguration.SecretName,
				URL:        ucn.Spec.BucketConfiguration.BucketURI,
				DestDir:    filepath.Join(n.UserCodeBucketPath, ucn.Name+".zip"),
			})
			if err != nil {
				logger.Error(err, "unable to download Jar file")
			}
			return err
		})
	}
	return g.Wait()
}

func (r *UserCodeNamespaceReconciler) applyConfig(ctx context.Context, ucn *hazelcastv1alpha1.UserCodeNamespace, client hzclient.Client, logger logr.Logger) error {
	logger.Info("Applying UserCodeNamespace config")
	service := hzclient.NewUsercodeNamespaceService(client)
	return service.Apply(ctx, ucn.Name)
}
