package hazelcast

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type topicOptionsBuilder struct {
	status         hazelcastv1alpha1.TopicConfigState
	err            error
	message        string
	retryAfter     time.Duration
	memberStatuses map[string]hazelcastv1alpha1.TopicConfigState
}

func topicFailedStatus(err error) topicOptionsBuilder {
	return topicOptionsBuilder{
		status: hazelcastv1alpha1.TopicFailed,
		err:    err,
	}
}

func topicSuccessStatus() topicOptionsBuilder {
	return topicOptionsBuilder{
		status: hazelcastv1alpha1.TopicSuccess,
	}
}

func topicPendingStatus(retryAfter time.Duration) topicOptionsBuilder {
	return topicOptionsBuilder{
		status:     hazelcastv1alpha1.TopicPending,
		retryAfter: retryAfter,
	}
}

func topicPersistingStatus(retryAfter time.Duration) topicOptionsBuilder {
	return topicOptionsBuilder{
		status:     hazelcastv1alpha1.TopicPersisting,
		retryAfter: retryAfter,
	}
}

func topicTerminatingStatus(err error) topicOptionsBuilder {
	return topicOptionsBuilder{
		status: hazelcastv1alpha1.TopicTerminating,
		err:    err,
	}
}

func (o topicOptionsBuilder) withMessage(m string) topicOptionsBuilder {
	o.message = m
	return o
}

func (o topicOptionsBuilder) withError(err error) topicOptionsBuilder {
	o.err = err
	return o
}

func (o topicOptionsBuilder) withMemberStatuses(m map[string]hazelcastv1alpha1.TopicConfigState) topicOptionsBuilder {
	o.memberStatuses = m
	return o
}

func updateTopicStatus(ctx context.Context, c client.Client, topic *hazelcastv1alpha1.Topic, options topicOptionsBuilder) (ctrl.Result, error) {
	topic.Status.State = options.status
	topic.Status.Message = options.message
	topic.Status.MemberStatuses = options.memberStatuses
	if err := c.Status().Update(ctx, topic); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if options.status == hazelcastv1alpha1.TopicFailed {
		return ctrl.Result{}, options.err
	}
	if options.status == hazelcastv1alpha1.TopicPending || options.status == hazelcastv1alpha1.TopicPersisting {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}
