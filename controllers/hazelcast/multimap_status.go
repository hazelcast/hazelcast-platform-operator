package hazelcast

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type multiMapOptionsBuilder struct {
	status         hazelcastv1alpha1.MultiMapConfigState
	err            error
	message        string
	retryAfter     time.Duration
	memberStatuses map[string]hazelcastv1alpha1.MultiMapConfigState
}

func mmFailedStatus(err error) multiMapOptionsBuilder {
	return multiMapOptionsBuilder{
		status: hazelcastv1alpha1.MultiMapFailed,
		err:    err,
	}
}

func mmSuccessStatus() multiMapOptionsBuilder {
	return multiMapOptionsBuilder{
		status: hazelcastv1alpha1.MultiMapSuccess,
	}
}

func mmPendingStatus(retryAfter time.Duration) multiMapOptionsBuilder {
	return multiMapOptionsBuilder{
		status:     hazelcastv1alpha1.MultiMapPending,
		retryAfter: retryAfter,
	}
}

func mmPersistingStatus(retryAfter time.Duration) multiMapOptionsBuilder {
	return multiMapOptionsBuilder{
		status:     hazelcastv1alpha1.MultiMapPersisting,
		retryAfter: retryAfter,
	}
}

func mmTerminatingStatus(err error) multiMapOptionsBuilder {
	return multiMapOptionsBuilder{
		status: hazelcastv1alpha1.MultiMapTerminating,
		err:    err,
	}
}

func (o multiMapOptionsBuilder) withMessage(m string) multiMapOptionsBuilder {
	o.message = m
	return o
}

func (o multiMapOptionsBuilder) withError(err error) multiMapOptionsBuilder {
	o.err = err
	return o
}

func (o multiMapOptionsBuilder) withMemberStatuses(m map[string]hazelcastv1alpha1.MultiMapConfigState) multiMapOptionsBuilder {
	o.memberStatuses = m
	return o
}

func updateMultiMapStatus(ctx context.Context, c client.Client, mm *hazelcastv1alpha1.MultiMap, options multiMapOptionsBuilder) (ctrl.Result, error) {
	mm.Status.State = options.status
	mm.Status.Message = options.message
	mm.Status.MemberStatuses = options.memberStatuses
	if err := c.Status().Update(ctx, mm); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if options.status == hazelcastv1alpha1.MultiMapFailed {
		return ctrl.Result{}, options.err
	}
	if options.status == hazelcastv1alpha1.MultiMapPending || options.status == hazelcastv1alpha1.MultiMapPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}
