package hazelcast

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type wanSyncOptionsBuilder struct {
	publisherId string
	phase       hazelcastv1alpha1.WanSyncPhase
	message     string
}

func wanSyncFailedStatus(err error) wanSyncOptionsBuilder {
	return wanSyncOptionsBuilder{
		phase:   hazelcastv1alpha1.WanSyncFailed,
		message: err.Error(),
	}
}

func wanSyncPendingStatus() wanSyncOptionsBuilder {
	return wanSyncOptionsBuilder{
		phase: hazelcastv1alpha1.WanSyncPending,
	}
}

func wanSyncSuccessStatus() wanSyncOptionsBuilder {
	return wanSyncOptionsBuilder{
		phase: hazelcastv1alpha1.WanSyncCompleted,
	}
}

func (o wanSyncOptionsBuilder) withPublisherId(id string) wanSyncOptionsBuilder {
	o.publisherId = id
	return o
}

func (o wanSyncOptionsBuilder) withMessage(msg string) wanSyncOptionsBuilder {
	o.message = msg
	return o
}

func updateWanSyncStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanSync, options wanSyncOptionsBuilder) (ctrl.Result, error) {
	wan.Status.Phase = options.phase
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
