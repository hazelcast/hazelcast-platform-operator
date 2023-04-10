package hazelcast

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers"
)

type WanRepStatusApplier interface {
	WanRepStatusApply(ms *hazelcastv1beta1.WanReplicationStatus)
}

type withWanRepState hazelcastv1beta1.WanStatus

func (w withWanRepState) WanRepStatusApply(ms *hazelcastv1beta1.WanReplicationStatus) {
	ms.Status = hazelcastv1beta1.WanStatus(w)
}

type withWanRepFailedState string

func (w withWanRepFailedState) WanRepStatusApply(ms *hazelcastv1beta1.WanReplicationStatus) {
	ms.Status = hazelcastv1beta1.WanStatusFailed
	ms.Message = string(w)
}

type withWanRepMessage string

func (w withWanRepMessage) WanRepStatusApply(ms *hazelcastv1beta1.WanReplicationStatus) {
	ms.Message = string(w)
}

func updateWanStatus(ctx context.Context, c client.Client, wan *hazelcastv1beta1.WanReplication, recOption controllers.ReconcilerOption, options ...WanRepStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.WanRepStatusApply(&wan.Status)
	}

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if wan.Status.Status == hazelcastv1beta1.WanStatusPending || wan.Status.Status == hazelcastv1beta1.WanStatusPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}

type WanMapStatusApplier interface {
	WanMapStatusApply(ms *hazelcastv1beta1.WanReplicationMapStatus)
}

type wanMapFailedStatus string

func (w wanMapFailedStatus) WanMapStatusApply(ms *hazelcastv1beta1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1beta1.WanStatusFailed
	ms.Message = string(w)
}

type wanMapPersistingStatus struct {
	publisherId, resourceName string
}

func (w wanMapPersistingStatus) WanMapStatusApply(ms *hazelcastv1beta1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1beta1.WanStatusPersisting
	ms.PublisherId = w.publisherId
	ms.Message = ""
	ms.ResourceName = w.resourceName
}

type wanMapSuccessStatus struct{}

func (w wanMapSuccessStatus) WanMapStatusApply(ms *hazelcastv1beta1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1beta1.WanStatusSuccess
}

type wanMapTerminatingStatus struct{}

func (w wanMapTerminatingStatus) WanMapStatusApply(ms *hazelcastv1beta1.WanReplicationMapStatus) {
	ms.Status = hazelcastv1beta1.WanStatusTerminating
}

func patchWanMapStatuses(ctx context.Context, c client.Client, wan *hazelcastv1beta1.WanReplication, options map[string]WanMapStatusApplier) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		wan.Status.WanReplicationMapsStatus = make(map[string]hazelcastv1beta1.WanReplicationMapStatus)
	}

	for mapWanKey, applier := range options {
		if _, ok := wan.Status.WanReplicationMapsStatus[mapWanKey]; !ok {
			wan.Status.WanReplicationMapsStatus[mapWanKey] = hazelcastv1beta1.WanReplicationMapStatus{}
		}
		status := wan.Status.WanReplicationMapsStatus[mapWanKey]
		applier.WanMapStatusApply(&status)
		wan.Status.WanReplicationMapsStatus[mapWanKey] = status
	}

	wan.Status.Status = wanStatusFromMapStatuses(wan.Status.WanReplicationMapsStatus)
	if wan.Status.Status == hazelcastv1beta1.WanStatusSuccess {
		wan.Status.Message = ""
	}

	if err := c.Status().Update(ctx, wan); err != nil {
		if errors.IsConflict(err) {
			return nil
		}
		return err
	}

	return nil
}

func wanStatusFromMapStatuses(statuses map[string]hazelcastv1beta1.WanReplicationMapStatus) hazelcastv1beta1.WanStatus {
	set := wanStatusSet(statuses, hazelcastv1beta1.WanStatusSuccess)

	_, successOk := set[hazelcastv1beta1.WanStatusSuccess]
	_, failOk := set[hazelcastv1beta1.WanStatusFailed]
	_, persistingOk := set[hazelcastv1beta1.WanStatusPersisting]

	if successOk && len(set) == 1 {
		return hazelcastv1beta1.WanStatusSuccess
	}

	if persistingOk {
		return hazelcastv1beta1.WanStatusPersisting
	}

	if failOk {
		return hazelcastv1beta1.WanStatusFailed
	}

	return hazelcastv1beta1.WanStatusPending

}

func wanStatusSet(statusMap map[string]hazelcastv1beta1.WanReplicationMapStatus, checkStatuses ...hazelcastv1beta1.WanStatus) map[hazelcastv1beta1.WanStatus]struct{} {
	statusSet := map[hazelcastv1beta1.WanStatus]struct{}{}

	for _, v := range statusMap {
		statusSet[v.Status] = struct{}{}
	}
	return statusSet
}

func deleteWanMapStatus(ctx context.Context, c client.Client, wan *hazelcastv1beta1.WanReplication, mapWanKey string) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		return nil
	}

	delete(wan.Status.WanReplicationMapsStatus, mapWanKey)

	if err := c.Status().Update(ctx, wan); err != nil {
		return err
	}

	return nil
}

func updateWanMapStatus(ctx context.Context, c client.Client, wan *hazelcastv1beta1.WanReplication, mapWanKey string, applier WanMapStatusApplier) error {
	if wan.Status.WanReplicationMapsStatus == nil {
		return nil
	}

	if _, ok := wan.Status.WanReplicationMapsStatus[mapWanKey]; !ok {
		wan.Status.WanReplicationMapsStatus[mapWanKey] = hazelcastv1beta1.WanReplicationMapStatus{}
	}
	status := wan.Status.WanReplicationMapsStatus[mapWanKey]
	applier.WanMapStatusApply(&status)
	wan.Status.WanReplicationMapsStatus[mapWanKey] = status

	if err := c.Status().Update(ctx, wan); err != nil {
		return err
	}

	return nil
}
