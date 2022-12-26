package hazelcast

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type wanOptionsBuilder struct {
	publisherId string
	err         error
	status      hazelcastv1alpha1.WanStatus
	message     string
	retryAfter  time.Duration
}

func wanFailedStatus(err error) wanOptionsBuilder {
	return wanOptionsBuilder{
		status: hazelcastv1alpha1.WanStatusFailed,
		err:    err,
	}
}

func wanPendingStatus() wanOptionsBuilder {
	return wanOptionsBuilder{
		status: hazelcastv1alpha1.WanStatusPending,
	}
}

func wanSuccessStatus() wanOptionsBuilder {
	return wanOptionsBuilder{
		status: hazelcastv1alpha1.WanStatusSuccess,
	}
}

func wanPersistingStatus(retryAfter time.Duration) wanOptionsBuilder {
	return wanOptionsBuilder{
		status:     hazelcastv1alpha1.WanStatusPersisting,
		retryAfter: retryAfter,
	}
}

func wanTerminatingStatus() wanOptionsBuilder {
	return wanOptionsBuilder{
		status: hazelcastv1alpha1.WanStatusTerminating,
	}
}

func wanStatus(statuses map[string]hazelcastv1alpha1.WanReplicationMapStatus) hazelcastv1alpha1.WanStatus {
	set := wanStatusSet(statuses, hazelcastv1alpha1.WanStatusSuccess)

	_, successOk := set[hazelcastv1alpha1.WanStatusSuccess]
	_, failOk := set[hazelcastv1alpha1.WanStatusFailed]
	_, persistingOk := set[hazelcastv1alpha1.WanStatusPersisting]

	if successOk && len(set) == 1 {
		return hazelcastv1alpha1.WanStatusSuccess
	}

	if persistingOk {
		return hazelcastv1alpha1.WanStatusPersisting
	}

	if failOk {
		return hazelcastv1alpha1.WanStatusFailed
	}

	return hazelcastv1alpha1.WanStatusPending

}

func wanStatusSet(statusMap map[string]hazelcastv1alpha1.WanReplicationMapStatus, checkStatuses ...hazelcastv1alpha1.WanStatus) map[hazelcastv1alpha1.WanStatus]struct{} {
	statusSet := map[hazelcastv1alpha1.WanStatus]struct{}{}

	for _, v := range statusMap {
		statusSet[v.Status] = struct{}{}
	}
	return statusSet
}

func (o wanOptionsBuilder) withPublisherId(hz string) wanOptionsBuilder {
	o.publisherId = hz
	return o
}

func (o wanOptionsBuilder) withMessage(msg string) wanOptionsBuilder {
	o.message = msg
	return o
}

func updateWanStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication, options wanOptionsBuilder) (ctrl.Result, error) {
	wan.Status.Status = options.status
	wan.Status.Message = options.message

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if options.status == hazelcastv1alpha1.WanStatusFailed {
		return ctrl.Result{}, options.err
	}
	if options.status == hazelcastv1alpha1.WanStatusPending || options.status == hazelcastv1alpha1.WanStatusPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}

func putWanMapStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanReplication, options map[string]wanOptionsBuilder) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		wan.Status.WanReplicationMapsStatus = make(map[string]hazelcastv1alpha1.WanReplicationMapStatus)
	}

	for mapWanKey, builder := range options {
		wan.Status.WanReplicationMapsStatus[mapWanKey] = hazelcastv1alpha1.WanReplicationMapStatus{
			PublisherId: builder.publisherId,
			Message:     builder.message,
			Status:      builder.status,
		}
	}

	wan.Status.Status = wanStatus(wan.Status.WanReplicationMapsStatus)

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return nil
		}
		return err
	}

	return nil
}
