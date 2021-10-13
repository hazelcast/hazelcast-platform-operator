package hazelcast

import (
	"context"
	"fmt"
	"time"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type optionsBuilder struct {
	phase        hazelcastv1alpha1.Phase
	retryAfter   time.Duration
	err          error
	readyMembers int
	message      string
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

func (o optionsBuilder) withReadyMembers(m int) optionsBuilder {
	o.readyMembers = m
	return o
}

func (o optionsBuilder) withMessage(m string) optionsBuilder {
	o.message = m
	return o
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Hazelcast resource
func update(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, options optionsBuilder) (ctrl.Result, error) {
	h.Status.Phase = options.phase
	h.Status.Cluster.ReadyMembers = fmt.Sprintf("%d/%d", options.readyMembers, h.Spec.ClusterSize)
	h.Status.Message = options.message
	if err := c.Status().Update(ctx, h); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
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
