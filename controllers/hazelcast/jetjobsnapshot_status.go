package hazelcast

import (
	"context"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type JetJobSnapshotStatusApplierFunc func(s *hazelcastv1alpha1.JetJobSnapshotStatus)

func withJetJobSnapshotState(state hazelcastv1alpha1.JetJobSnapshotState) JetJobSnapshotStatusApplierFunc {
	return func(s *hazelcastv1alpha1.JetJobSnapshotStatus) {
		s.State = state
	}
}

func withJetJobSnapshotFailedState(message string) JetJobSnapshotStatusApplierFunc {
	return func(s *hazelcastv1alpha1.JetJobSnapshotStatus) {
		s.State = hazelcastv1alpha1.JetJobSnapshotFailed
		s.Message = message
	}
}

func updateJetJobSnapshotStatus(ctx context.Context, c client.Client, jjs *hazelcastv1alpha1.JetJobSnapshot,
	recOption controllers.ReconcilerOption, options ...JetJobSnapshotStatusApplierFunc) (ctrl.Result, error) {

	for _, applierFunc := range options {
		applierFunc(&jjs.Status)
	}

	if err := c.Status().Update(ctx, jjs); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}

	return ctrl.Result{}, nil
}

func updateJetJobSnapshotStatusRetry(ctx context.Context, c client.Client, jjs *hazelcastv1alpha1.JetJobSnapshot,
	options ...JetJobSnapshotStatusApplierFunc) error {

	for _, applierFunc := range options {
		applierFunc(&jjs.Status)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return c.Status().Update(ctx, jjs)
	})
}
