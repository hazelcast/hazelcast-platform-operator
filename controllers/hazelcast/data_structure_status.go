package hazelcast

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

const retryAfterForDataStructures = 5 * time.Second

type DataStructureOptionsBuilder struct {
	Status         hazelcastv1alpha1.DataStructureConfigState
	err            error
	Message        string
	retryAfter     time.Duration
	MemberStatuses map[string]hazelcastv1alpha1.DataStructureConfigState
}

func dsFailedStatus(err error) DataStructureOptionsBuilder {
	return DataStructureOptionsBuilder{
		Status: hazelcastv1alpha1.DataStructureFailed,
		err:    err,
	}
}

func dsSuccessStatus() DataStructureOptionsBuilder {
	return DataStructureOptionsBuilder{
		Status: hazelcastv1alpha1.DataStructureSuccess,
	}
}

func dsPendingStatus(retryAfter time.Duration) DataStructureOptionsBuilder {
	return DataStructureOptionsBuilder{
		Status:     hazelcastv1alpha1.DataStructurePending,
		retryAfter: retryAfter,
	}
}

func dsPersistingStatus(retryAfter time.Duration) DataStructureOptionsBuilder {
	return DataStructureOptionsBuilder{
		Status:     hazelcastv1alpha1.DataStructurePersisting,
		retryAfter: retryAfter,
	}
}

func dsTerminatingStatus(err error) DataStructureOptionsBuilder {
	return DataStructureOptionsBuilder{
		Status: hazelcastv1alpha1.DataStructureTerminating,
		err:    err,
	}
}

func (o DataStructureOptionsBuilder) withMessage(m string) DataStructureOptionsBuilder {
	o.Message = m
	return o
}

func (o DataStructureOptionsBuilder) withError(err error) DataStructureOptionsBuilder {
	o.err = err
	return o
}

func (o DataStructureOptionsBuilder) withMemberStatuses(m map[string]hazelcastv1alpha1.DataStructureConfigState) DataStructureOptionsBuilder {
	o.MemberStatuses = m
	return o
}

func updateDSStatus(ctx context.Context, c client.Client, obj client.Object, options DataStructureOptionsBuilder) (ctrl.Result, error) {

	obj.(DataStructure).SetStatus(options.Status, options.Message, options.MemberStatuses)
	if err := c.Status().Update(ctx, obj); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	dsStatus := obj.(DataStructure).GetStatus()
	if dsStatus == hazelcastv1alpha1.DataStructureFailed {
		return ctrl.Result{}, options.err
	}
	if dsStatus == hazelcastv1alpha1.DataStructurePending || dsStatus == hazelcastv1alpha1.DataStructurePersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}
