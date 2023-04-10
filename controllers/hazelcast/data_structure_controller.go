package hazelcast

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	recoptions "github.com/hazelcast/hazelcast-platform-operator/controllers"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type CRLister interface {
	GetItems() []client.Object
}

type Type interface {
	GetKind() string
}

type DataStructure interface {
	GetDSName() string
	GetHZResourceName() string
	GetStatus() *hazelcastv1beta1.DataStructureStatus
	GetSpec() (string, error)
	SetSpec(string) error
}

type Update func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error

func isDSPersisted(obj client.Object) bool {
	switch obj.(DataStructure).GetStatus().State {
	case hazelcastv1beta1.DataStructurePersisting, hazelcastv1beta1.DataStructureSuccess:
		return true
	case hazelcastv1beta1.DataStructureFailed, hazelcastv1beta1.DataStructurePending:
		if spec, ok := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]; ok {
			if err := obj.(DataStructure).SetSpec(spec); err != nil {
				return false
			}
			return true
		}
	}
	return false
}

func initialSetupDS(ctx context.Context,
	c client.Client,
	nn client.ObjectKey,
	obj client.Object,
	updateFunc Update,
	cs hzclient.ClientRegistry,
	logger logr.Logger) (hzclient.Client, ctrl.Result, error) {
	if err := getCR(ctx, c, obj, nn, logger); err != nil {
		return nil, ctrl.Result{}, err
	}
	objKind := obj.(Type).GetKind()

	if err := util.AddFinalizer(ctx, c, obj, logger); err != nil {
		result, err := updateDSStatus(ctx, c, obj, recoptions.Error(err),
			withDSFailedState(err.Error()))
		return nil, result, err
	}

	if obj.GetDeletionTimestamp() != nil {
		if err := startDelete(ctx, c, obj, updateFunc, logger); err != nil {
			result, err := updateDSStatus(ctx, c, obj, recoptions.Error(err),
				withDSState(hazelcastv1beta1.DataStructureTerminating),
				withDSMessage(err.Error()))
			return nil, result, err
		}
		return nil, ctrl.Result{}, nil
	}

	cont, res, err := handleCreatedBefore(ctx, c, obj, logger)
	if !cont {
		return nil, res, err
	}

	cl, res, err := getHZClient(ctx, c, obj, nn.Namespace, obj.(DataStructure).GetHZResourceName(), cs)
	if cl == nil {
		return nil, res, err
	}

	if obj.(DataStructure).GetStatus().State != hazelcastv1beta1.DataStructurePersisting {
		requeue, err := updateDSStatus(ctx, c, obj, recoptions.Empty(),
			withDSState(hazelcastv1beta1.Pending),
			withDSMessage(fmt.Sprintf("Applying new %v configuration.", objKind)))
		if err != nil {
			return nil, requeue, err
		}
	}
	return cl, ctrl.Result{}, nil
}

func getCR(ctx context.Context, c client.Client, obj client.Object, nn client.ObjectKey, logger logr.Logger) error {
	err := c.Get(ctx, nn, obj)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Resource not found. Ignoring since object must be deleted")
			return err
		}
		return fmt.Errorf("failed to get resource: %w", err)
	}
	return nil
}

func startDelete(ctx context.Context, c client.Client, obj client.Object, updateFunc Update, logger logr.Logger) error {
	updateDSStatus(ctx, c, obj, recoptions.Empty(), withDSState(hazelcastv1beta1.Terminating)) //nolint:errcheck
	if err := executeFinalizer(ctx, obj, updateFunc); err != nil {
		return err
	}
	logger.V(util.DebugLevel).Info("Finalizer's pre-delete function executed successfully and the finalizer removed from custom resource", "Name:", n.Finalizer)
	return nil
}

func executeFinalizer(ctx context.Context, obj client.Object, updateFunc Update) error {
	if !controllerutil.ContainsFinalizer(obj, n.Finalizer) {
		return nil
	}
	controllerutil.RemoveFinalizer(obj, n.Finalizer)
	err := updateFunc(ctx, obj)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func handleCreatedBefore(ctx context.Context, c client.Client, obj client.Object, logger logr.Logger) (bool, ctrl.Result, error) {
	s, createdBefore := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]
	objKind := obj.(Type).GetKind()
	if createdBefore {
		mms, err := obj.(DataStructure).GetSpec()
		if err != nil {
			result, err := updateDSStatus(ctx, c, obj, recoptions.Error(err),
				withDSFailedState(err.Error()))
			return false, result, err
		}

		if s == mms {
			logger.Info(fmt.Sprintf("%v Config was already applied.", objKind), "name", obj.GetName(), "namespace", obj.GetNamespace())
			result, err := updateDSStatus(ctx, c, obj, recoptions.Empty(), withDSState(hazelcastv1beta1.DataStructureSuccess))
			return false, result, err
		}

		returnErr := fmt.Errorf("%v is not updatable", objKind)
		result, err := updateDSStatus(ctx, c, obj, recoptions.Error(returnErr),
			withDSFailedState(returnErr.Error()))
		return false, result, err
	}
	return true, ctrl.Result{}, nil
}

func getHZClient(ctx context.Context, c client.Client, obj client.Object, ns, hzResourceName string, cs hzclient.ClientRegistry) (hzclient.Client, ctrl.Result, error) {
	h := &hazelcastv1beta1.Hazelcast{}
	hzLookup := types.NamespacedName{Namespace: ns, Name: hzResourceName}
	err := c.Get(ctx, hzLookup, h)
	if err != nil {
		err = fmt.Errorf("could not create/update %v config: Hazelcast resource not found: %w", obj.(Type).GetKind(), err)
		result, err := updateDSStatus(ctx, c, obj, recoptions.Error(err),
			withDSFailedState(err.Error()))
		return nil, result, err
	}
	if h.Status.Phase != hazelcastv1beta1.Running {
		err = errors.NewServiceUnavailable("Hazelcast CR is not ready")
		result, err := updateDSStatus(ctx, c, obj, recoptions.Error(err),
			withDSFailedState(err.Error()))
		return nil, result, err
	}

	hzcl, err := cs.GetOrCreate(ctx, hzLookup)
	if err != nil {
		err = errors.NewInternalError(fmt.Errorf("cannot connect to the cluster for %s", hzResourceName))
		result, err := updateDSStatus(ctx, c, obj, recoptions.Error(err),
			withDSFailedState(err.Error()))
		return nil, result, err
	}
	if !hzcl.Running() {
		err = fmt.Errorf("trying to connect to the cluster %s", hzResourceName)
		result, err := updateDSStatus(ctx, c, obj, recoptions.RetryAfter(retryAfterForDataStructures),
			withDSState(hazelcastv1beta1.Pending),
			withDSMessage(err.Error()))
		return nil, result, err
	}

	return hzcl, ctrl.Result{}, nil
}

func finalSetupDS(ctx context.Context, c client.Client, ph chan struct{}, obj client.Object, logger logr.Logger) (ctrl.Result, error) {
	if util.IsPhoneHomeEnabled() && !util.IsSuccessfullyApplied(obj) {
		go func() { ph <- struct{}{} }()
	}

	err := updateLastSuccessfulConfigurationDS(ctx, c, obj, logger)
	if err != nil {
		logger.Info("Could not save the current successful spec as annotation to the custom resource")
	}

	return updateDSStatus(ctx, c, obj, recoptions.Empty(),
		withDSState(hazelcastv1beta1.DataStructureSuccess),
		withDSMessage(""),
		withDSMemberStatuses{})
}

func updateLastSuccessfulConfigurationDS(ctx context.Context, c client.Client, obj client.Object, logger logr.Logger) error {
	spec, err := obj.(DataStructure).GetSpec()
	if err != nil {
		return err
	}
	opResult, err := util.Update(ctx, c, obj, func() error {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[n.LastSuccessfulSpecAnnotation] = spec
		obj.SetAnnotations(annotations)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", fmt.Sprintf("%v Annotation", obj.(Type).GetKind()), obj.GetName(), "result", opResult)
	}
	return err
}

func sendCodecRequest(
	ctx context.Context,
	cl hzclient.Client,
	obj client.Object,
	req *hazelcast.ClientMessage,
	logger logr.Logger) (map[string]hazelcastv1beta1.DataStructureConfigState, error) {
	memberStatuses := map[string]hazelcastv1beta1.DataStructureConfigState{}
	var failedMembers strings.Builder
	for _, member := range cl.OrderedMembers() {
		if status, ok := obj.(DataStructure).GetStatus().MemberStatuses[member.UUID.String()]; ok && status == hazelcastv1beta1.DataStructureSuccess {
			memberStatuses[member.UUID.String()] = hazelcastv1beta1.DataStructureSuccess
			continue
		}
		_, err := cl.InvokeOnMember(ctx, req, member.UUID, nil)
		if err != nil {
			memberStatuses[member.UUID.String()] = hazelcastv1beta1.DataStructureFailed
			failedMembers.WriteString(member.UUID.String() + ", ")
			logger.Error(err, "Failed with member")
			continue
		}
		memberStatuses[member.UUID.String()] = hazelcastv1beta1.DataStructureSuccess
	}
	errString := failedMembers.String()
	if errString != "" {
		return memberStatuses, fmt.Errorf("error creating/updating the MultiMap config %s for members %s", obj.(DataStructure).GetDSName(), errString[:len(errString)-2])
	}

	return memberStatuses, nil
}

func getHazelcastConfig(ctx context.Context, c client.Client, obj client.Object) (*config.HazelcastWrapper, error) {
	cm := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: obj.(DataStructure).GetHZResourceName(), Namespace: obj.GetNamespace()}, cm)
	if err != nil {
		return nil, fmt.Errorf("could not find Secret for %v config persistence", obj.(Type).GetKind())
	}

	hzConfig := &config.HazelcastWrapper{}
	err = yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
	if err != nil {
		return nil, fmt.Errorf("persisted Secret is not formatted correctly")
	}
	return hzConfig, nil
}
