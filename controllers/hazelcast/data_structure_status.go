package hazelcast

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers"
)

const retryAfterForDataStructures = 5 * time.Second

type DSStatusApplier interface {
	DSStatusApply(ds *hazelcastv1beta1.DataStructureStatus)
}

type withDSState hazelcastv1beta1.DataStructureConfigState

func (w withDSState) DSStatusApply(ds *hazelcastv1beta1.DataStructureStatus) {
	ds.State = hazelcastv1beta1.DataStructureConfigState(w)
	if hazelcastv1beta1.DataStructureConfigState(w) == hazelcastv1beta1.DataStructureSuccess {
		ds.Message = ""
	}
}

type withDSFailedState string

func (w withDSFailedState) DSStatusApply(ds *hazelcastv1beta1.DataStructureStatus) {
	ds.State = hazelcastv1beta1.DataStructureFailed
	ds.Message = string(w)
}

type withDSMessage string

func (w withDSMessage) DSStatusApply(ds *hazelcastv1beta1.DataStructureStatus) {
	ds.Message = string(w)
}

type withDSMemberStatuses map[string]hazelcastv1beta1.DataStructureConfigState

func (w withDSMemberStatuses) DSStatusApply(ds *hazelcastv1beta1.DataStructureStatus) {
	ds.MemberStatuses = w
}

func updateDSStatus(ctx context.Context, c client.Client, obj client.Object, recOption controllers.ReconcilerOption, options ...DSStatusApplier) (ctrl.Result, error) {
	status := obj.(DataStructure).GetStatus()
	for _, applier := range options {
		applier.DSStatusApply(status)
	}
	if err := c.Status().Update(ctx, obj); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	dsStatus := obj.(DataStructure).GetStatus().State
	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if dsStatus == hazelcastv1beta1.DataStructurePending || dsStatus == hazelcastv1beta1.DataStructurePersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}
