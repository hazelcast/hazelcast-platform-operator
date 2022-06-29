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
	wan.Status.PublisherId = options.publisherId
	wan.Status.Message = options.message

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
