package managementcenter

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers"
)

type McStatusApplier interface {
	McStatusApply(ms *hazelcastv1beta1.ManagementCenterStatus)
}

type withMcPhase hazelcastv1beta1.Phase

func (w withMcPhase) McStatusApply(ms *hazelcastv1beta1.ManagementCenterStatus) {
	ms.Phase = hazelcastv1beta1.Phase(w)
	if hazelcastv1beta1.Phase(w) == hazelcastv1beta1.Running {
		ms.Message = ""
	}
}

type withMcFailedPhase string

func (w withMcFailedPhase) McStatusApply(ms *hazelcastv1beta1.ManagementCenterStatus) {
	ms.Phase = hazelcastv1beta1.Failed
	ms.Message = string(w)

}

type withMcExternalAddresses []string

func (w withMcExternalAddresses) McStatusApply(ms *hazelcastv1beta1.ManagementCenterStatus) {
	ms.ExternalAddresses = strings.Join(w, ",")
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Management Center resource
func update(ctx context.Context, c client.Client, mc *hazelcastv1beta1.ManagementCenter, recOption controllers.ReconcilerOption, options ...McStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.McStatusApply(&mc.Status)
	}

	if err := c.Status().Update(ctx, mc); err != nil {
		return ctrl.Result{}, err
	}
	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if mc.Status.Phase == hazelcastv1beta1.Pending {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}
