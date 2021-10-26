package managementcenter

import (
	"context"
	"time"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type optionsBuilder struct {
	phase      hazelcastv1alpha1.Phase
	retryAfter time.Duration
	err        error
}

func failedPhase(err error) optionsBuilder {
	return optionsBuilder{
		phase: hazelcastv1alpha1.Failed,
		err:   err,
	}
}

func pendingPhase(retryAfter time.Duration) optionsBuilder {
	return optionsBuilder{
		phase:      hazelcastv1alpha1.Pending,
		retryAfter: retryAfter,
	}
}

func runningPhase() optionsBuilder {
	return optionsBuilder{
		phase: hazelcastv1alpha1.Running,
	}
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Management Center resource
func update(ctx context.Context, statusWriter client.StatusWriter, mc *hazelcastv1alpha1.ManagementCenter, options optionsBuilder) (ctrl.Result, error) {
	mc.Status = hazelcastv1alpha1.ManagementCenterStatus{Phase: options.phase}
	if err := statusWriter.Update(ctx, mc); err != nil {
		return ctrl.Result{}, err
	}
	if options.phase == hazelcastv1alpha1.Failed {
		return ctrl.Result{}, options.err
	}
	if options.phase == hazelcastv1alpha1.Pending {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}
