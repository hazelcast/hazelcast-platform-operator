package hazelcast

import (
	"context"
	"errors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers"
)

type HazelcastEndpointStatusApplierFunc = func(s *hazelcastv1alpha1.HazelcastEndpointStatus)

func withHazelcastEndpointMessage(message string) HazelcastEndpointStatusApplierFunc {
	return func(s *hazelcastv1alpha1.HazelcastEndpointStatus) {
		s.Message = message
		s.Address = ""
	}
}

func withHazelcastEndpointMessageByError(err error) HazelcastEndpointStatusApplierFunc {
	return func(s *hazelcastv1alpha1.HazelcastEndpointStatus) {
		if err != nil {
			withHazelcastEndpointMessage(err.Error())(s)
		} else {
			s.Message = ""
		}
	}
}

func withHazelcastEndpointAddress(address string) HazelcastEndpointStatusApplierFunc {
	return func(s *hazelcastv1alpha1.HazelcastEndpointStatus) {
		s.Address = address
		if address != "" {
			s.Message = ""
		}
	}
}

// updateStatus takes the options, applies them all, and then updates the HazelcastEndpoint resource
func (r *HazelcastEndpointReconciler) updateStatus(ctx context.Context, hzep *hazelcastv1alpha1.HazelcastEndpoint,
	recOption controllers.ReconcilerOption, options ...HazelcastEndpointStatusApplierFunc) (ctrl.Result, error) {

	for _, applierFunc := range options {
		applierFunc(&hzep.Status)
	}

	if err := r.Client.Status().Update(ctx, hzep); err != nil {
		if kerrors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if recOption.Err != nil && !errors.Is(recOption.Err, errorToGetExternalAddressOfHazelcastEndpointService) {
		return ctrl.Result{}, recOption.Err
	}

	return ctrl.Result{}, nil
}
