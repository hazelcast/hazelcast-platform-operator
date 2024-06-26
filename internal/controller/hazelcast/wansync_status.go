package hazelcast

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type wanSyncOptionsBuilder struct {
	phase        hazelcastv1alpha1.WanSyncPhase
	message      string
	mapsStatuses map[string]wanSyncMapStatus
}

type wanSyncMapStatus struct {
	phase        hazelcastv1alpha1.WanSyncPhase
	message      string
	publisherId  string
	resourceName string
}

func wanSyncStatus(hzMaps map[string][]hazelcastv1alpha1.Map) wanSyncOptionsBuilder {
	builder := wanSyncOptionsBuilder{
		phase:        hazelcastv1alpha1.WanSyncNotStarted,
		mapsStatuses: make(map[string]wanSyncMapStatus),
	}
	for hz, maps := range hzMaps {
		for _, m := range maps {
			builder.mapsStatuses[wanMapKey(hz, m.MapName())] = wanSyncMapStatus{phase: hazelcastv1alpha1.WanSyncNotStarted}
		}
	}
	return builder
}

func wanSyncFailedStatus(err error) wanSyncOptionsBuilder {
	return wanSyncOptionsBuilder{
		phase:        hazelcastv1alpha1.WanSyncFailed,
		message:      err.Error(),
		mapsStatuses: make(map[string]wanSyncMapStatus),
	}
}

func wanSyncPendingStatus() wanSyncOptionsBuilder {
	return wanSyncOptionsBuilder{
		phase:        hazelcastv1alpha1.WanSyncPending,
		mapsStatuses: make(map[string]wanSyncMapStatus),
	}
}

func updateWanSyncMapStatus(ctx context.Context, c client.Client, name types.NamespacedName, wanMapKey string, mapStatus wanSyncMapStatus) error {
	wan := &hazelcastv1alpha1.WanSync{}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := c.Get(ctx, name, wan); err != nil {
			return err
		}
		if wan.Status.WanSyncMapsStatus == nil {
			wan.Status.WanSyncMapsStatus = make(map[string]hazelcastv1alpha1.WanSyncMapStatus)
		}
		wan.Status.WanSyncMapsStatus[wanMapKey] = hazelcastv1alpha1.WanSyncMapStatus{
			Phase:        mapStatus.phase,
			ResourceName: mapStatus.resourceName,
			PublisherId:  mapStatus.publisherId,
			Message:      mapStatus.message,
		}
		if mapStatus.phase != hazelcastv1alpha1.WanSyncCompleted {
			wan.Status.Status = mapStatus.phase
			wan.Status.Message = mapStatus.message
		} else {
			st := hazelcastv1alpha1.WanSyncCompleted
			msg := ""
			for _, ms := range wan.Status.WanSyncMapsStatus {
				if ms.Phase != hazelcastv1alpha1.WanSyncCompleted {
					st = ms.Phase
					msg = ms.Message
				}
			}
			wan.Status.Status = st
			wan.Status.Message = msg
		}
		return updateStatus(ctx, c, wan)
	})
}

func updateWanSyncStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanSync, options wanSyncOptionsBuilder) (ctrl.Result, error) {
	wan.Status.Status = options.phase
	wan.Status.Message = options.message

	if wan.Status.WanSyncMapsStatus == nil {
		wan.Status.WanSyncMapsStatus = make(map[string]hazelcastv1alpha1.WanSyncMapStatus)
	}
	for key, status := range options.mapsStatuses {
		wan.Status.WanSyncMapsStatus[key] = hazelcastv1alpha1.WanSyncMapStatus{
			ResourceName: status.resourceName,
			PublisherId:  status.publisherId,
			Phase:        status.phase,
			Message:      status.message,
		}
	}

	err := updateStatus(ctx, c, wan)
	if options.phase == hazelcastv1alpha1.WanSyncPending {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, err
}

func updateStatus(ctx context.Context, c client.Client, wan *hazelcastv1alpha1.WanSync) error {
	return c.Status().Update(ctx, wan)
}
