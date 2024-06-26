package hazelcast

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type userCodeNamepsaceOptionsBuilder struct {
	state   hazelcastv1alpha1.UserCodeNamespaceState
	message string
}

func userCodeNamepsaceFailedStatus(err error) userCodeNamepsaceOptionsBuilder {
	return userCodeNamepsaceOptionsBuilder{
		state:   hazelcastv1alpha1.UserCodeNamespaceFailure,
		message: err.Error(),
	}
}

func userCodeNamepsacePendingStatus() userCodeNamepsaceOptionsBuilder {
	return userCodeNamepsaceOptionsBuilder{
		state: hazelcastv1alpha1.UserCodeNamespacePending,
	}
}

func userCodeNamespaceSuccessStatus() userCodeNamepsaceOptionsBuilder {
	return userCodeNamepsaceOptionsBuilder{
		state: hazelcastv1alpha1.UserCodeNamespaceSuccess,
	}
}

func updateUserCodeNamespaceStatus(ctx context.Context, c client.Client, ucn *hazelcastv1alpha1.UserCodeNamespace, options userCodeNamepsaceOptionsBuilder) (ctrl.Result, error) {
	ucn.Status.State = options.state
	ucn.Status.Message = options.message

	err := c.Status().Update(ctx, ucn)
	if options.state == hazelcastv1alpha1.UserCodeNamespacePending {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, err
}
