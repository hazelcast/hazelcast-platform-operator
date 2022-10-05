package hazelcast

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type wanOptionsBuilder struct {
	publisherId string
	status      hazelcastv1alpha1.WanStatus
	message     string
}

func wanFailedStatus() wanOptionsBuilder {
	return wanOptionsBuilder{
		status: hazelcastv1alpha1.WanStatusFailed,
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

func wanTerminatingStatus() wanOptionsBuilder {
	return wanOptionsBuilder{
		status: hazelcastv1alpha1.WanStatusTerminating,
	}
}

func (o wanOptionsBuilder) withPublisherId(id string) wanOptionsBuilder {
	o.publisherId = id
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

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return nil
		}
		return err
	}

	return nil
}

func isWanSuccessful(wan *hazelcastv1alpha1.WanReplication) bool {
	for _, mapStatus := range wan.Status.WanReplicationMapsStatus {
		if mapStatus.Status != hazelcastv1alpha1.WanStatusSuccess {
			return false
		}
	}
	return true
}
