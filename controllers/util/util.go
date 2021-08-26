package util

import (
	"context"
	"fmt"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	opResult, err := controllerutil.CreateOrUpdate(ctx, c, obj, f)
	if errors.IsAlreadyExists(err) {
		// Ignore "already exists" error.
		// Inside createOrUpdate() there's is a race condition between Get() and Create(), so this error is expected from time to time.
		return opResult, nil
	}
	return opResult, err
}

func HazelcastDockerImage(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s:%s", h.Spec.Repository, h.Spec.Version)
}

func MCDockerImage(mc *hazelcastv1alpha1.ManagementCenter) string {
	return fmt.Sprintf("%s:%s", mc.Spec.Repository, mc.Spec.Version)
}
