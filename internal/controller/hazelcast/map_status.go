package hazelcast

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
)

type MapStatusApplier interface {
	MapStatusApply(ms *hazelcastv1alpha1.MapStatus)
}

type withMapState hazelcastv1alpha1.MapConfigState

func (w withMapState) MapStatusApply(ms *hazelcastv1alpha1.MapStatus) {
	ms.State = hazelcastv1alpha1.MapConfigState(w)
	if hazelcastv1alpha1.MapConfigState(w) == hazelcastv1alpha1.MapSuccess {
		ms.Message = ""
	}
}

type withMapFailedState string

func (w withMapFailedState) MapStatusApply(ms *hazelcastv1alpha1.MapStatus) {
	ms.State = hazelcastv1alpha1.MapFailed
	ms.Message = string(w)
}

type withMapMessage string

func (w withMapMessage) MapStatusApply(ms *hazelcastv1alpha1.MapStatus) {
	ms.Message = string(w)
}

type withMapMemberStatuses map[string]hazelcastv1alpha1.MapConfigState

func (w withMapMemberStatuses) MapStatusApply(ms *hazelcastv1alpha1.MapStatus) {
	ms.MemberStatuses = w
}

func updateMapStatus(ctx context.Context, c client.Client, m *hazelcastv1alpha1.Map, recOption controller.ReconcilerOption, options ...MapStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.MapStatusApply(&m.Status)
	}

	if err := c.Status().Update(ctx, m); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if m.Status.State == hazelcastv1alpha1.MapPending || m.Status.State == hazelcastv1alpha1.MapPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}
