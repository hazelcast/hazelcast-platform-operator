package managementcenter

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	controllers "github.com/hazelcast/hazelcast-platform-operator/internal/controller"
)

type McStatusApplier interface {
	McStatusApply(ms *hazelcastv1alpha1.ManagementCenterStatus)
}

type withMcPhase hazelcastv1alpha1.MCPhase

func (w withMcPhase) McStatusApply(ms *hazelcastv1alpha1.ManagementCenterStatus) {
	ms.Phase = hazelcastv1alpha1.MCPhase(w)
	if hazelcastv1alpha1.MCPhase(w) == hazelcastv1alpha1.McRunning {
		ms.Message = ""
	}
}

type withConfigured bool

func (w withConfigured) McStatusApply(ms *hazelcastv1alpha1.ManagementCenterStatus) {
	ms.Configured = bool(w)
}

type withMcFailedPhase string

func (w withMcFailedPhase) McStatusApply(ms *hazelcastv1alpha1.ManagementCenterStatus) {
	ms.Phase = hazelcastv1alpha1.McFailed
	ms.Message = string(w)

}

type withMcExternalAddresses []string

func (w withMcExternalAddresses) McStatusApply(ms *hazelcastv1alpha1.ManagementCenterStatus) {
	ms.ExternalAddresses = strings.Join(w, ",")
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Management Center resource
func update(ctx context.Context, c client.Client, mc *hazelcastv1alpha1.ManagementCenter, recOption controllers.ReconcilerOption, options ...McStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.McStatusApply(&mc.Status)
	}

	if err := c.Status().Update(ctx, mc); err != nil {
		return ctrl.Result{}, err
	}
	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if mc.Status.Phase == hazelcastv1alpha1.McPending {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}
