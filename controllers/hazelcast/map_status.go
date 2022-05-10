package hazelcast

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type mapOptionsBuilder struct {
	status         hazelcastv1alpha1.MapConfigState
	err            error
	message        string
	retryAfter     time.Duration
	memberStatuses map[string]hazelcastv1alpha1.MapConfigState
}

func failedStatus(err error) mapOptionsBuilder {
	return mapOptionsBuilder{
		status: hazelcastv1alpha1.MapFailed,
		err:    err,
	}
}

func successStatus() mapOptionsBuilder {
	return mapOptionsBuilder{
		status: hazelcastv1alpha1.MapSuccess,
	}
}

func pendingStatus(retryAfter time.Duration) mapOptionsBuilder {
	return mapOptionsBuilder{
		status:     hazelcastv1alpha1.MapPending,
		retryAfter: retryAfter,
	}
}

func persistingStatus(retryAfter time.Duration) mapOptionsBuilder {
	return mapOptionsBuilder{
		status:     hazelcastv1alpha1.MapPersisting,
		retryAfter: retryAfter,
	}
}

func (o mapOptionsBuilder) withMessage(m string) mapOptionsBuilder {
	o.message = m
	return o
}

func (o mapOptionsBuilder) withError(err error) mapOptionsBuilder {
	o.err = err
	return o
}

func (o mapOptionsBuilder) withMemberStatuses(m map[string]hazelcastv1alpha1.MapConfigState) mapOptionsBuilder {
	o.memberStatuses = m
	return o
}

func updateMapStatus(ctx context.Context, c client.Client, m *hazelcastv1alpha1.Map, options mapOptionsBuilder) (ctrl.Result, error) {
	m.Status.State = options.status
	m.Status.Message = options.message
	m.Status.MemberStatuses = options.memberStatuses
	if err := c.Status().Update(ctx, m); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if options.status == hazelcastv1alpha1.MapFailed {
		return ctrl.Result{}, options.err
	}
	if options.status == hazelcastv1alpha1.MapPending || options.status == hazelcastv1alpha1.MapPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}
