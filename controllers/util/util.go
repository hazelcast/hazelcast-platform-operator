package util

import (
	"context"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

func CheckIfRunning(ctx context.Context, cl client.Client, namespacedName types.NamespacedName, expectedReplicas int32) bool {
	sts := &appsv1.StatefulSet{}
	err := cl.Get(ctx, client.ObjectKey{Name: namespacedName.Name, Namespace: namespacedName.Namespace}, sts)
	if err != nil {
		return false
	}
	return isStatefulSetReady(sts, expectedReplicas)
}

func isStatefulSetReady(sts *appsv1.StatefulSet, expectedReplicas int32) bool {
	allUpdated := expectedReplicas == sts.Status.UpdatedReplicas
	allReady := expectedReplicas == sts.Status.ReadyReplicas
	atExpectedGeneration := sts.Generation == sts.Status.ObservedGeneration
	return allUpdated && allReady && atExpectedGeneration
}

func IsEnterprise(repo string) bool {
	path := strings.Split(repo, "/")
	if len(path) == 0 {
		return false
	}
	return strings.HasSuffix(path[len(path)-1], "-enterprise")
}
