package util

import (
	"context"
	"fmt"
	"os"
	"strings"

	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

func CheckIfRunning(ctx context.Context, cl client.Client, namespacedName types.NamespacedName, expectedReplicas int32) (bool, error) {
	sts := &appsv1.StatefulSet{}
	err := cl.Get(ctx, client.ObjectKey{Name: namespacedName.Name, Namespace: namespacedName.Namespace}, sts)
	if err != nil {
		return false, err
	}
	if isStatefulSetReady(sts, expectedReplicas) {
		return true, nil
	}
	if err := checkPodsForFailure(ctx, cl, sts); err != nil {
		return false, err
	}
	return false, nil
}

func isStatefulSetReady(sts *appsv1.StatefulSet, expectedReplicas int32) bool {
	allUpdated := expectedReplicas == sts.Status.UpdatedReplicas
	allReady := expectedReplicas == sts.Status.ReadyReplicas
	atExpectedGeneration := sts.Generation == sts.Status.ObservedGeneration
	return allUpdated && allReady && atExpectedGeneration
}

func checkPodsForFailure(ctx context.Context, cl client.Client, sts *appsv1.StatefulSet) error {
	pods, err := listPods(ctx, cl, sts)
	if err != nil {
		return err
	}
	errs := make(PodErrors, 0, pods.Size())
	for _, pod := range pods.Items {
		phase := pod.Status.Phase
		if phase == corev1.PodFailed || phase == corev1.PodUnknown {
			errs = append(errs, NewPodError(&pod))
		} else if hasPodFailedWhilePending(&pod) {
			errs = append(errs, errorsFromPendingPod(&pod)...)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func hasPodFailedWhilePending(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodPending {
		return false
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			switch status.State.Waiting.Reason {
			case "ContainerCreating", "PodInitializing", "":
			default:
				return true
			}
		}
	}
	return false
}

func errorsFromPendingPod(pod *corev1.Pod) PodErrors {
	podErrors := make(PodErrors, 0, len(pod.Spec.Containers))
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			switch status.State.Waiting.Reason {
			case "ContainerCreating", "PodInitializing", "":
			default:
				podErrors = append(podErrors, &PodError{
					Name:      pod.Name,
					Namespace: pod.Namespace,
					Message:   status.State.Waiting.Message,
					Reason:    status.State.Waiting.Reason,
				})
			}
		}
	}
	return podErrors
}

func listPods(ctx context.Context, cl client.Client, sts *appsv1.StatefulSet) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	podLabels := sts.Spec.Template.Labels
	if err := cl.List(ctx, pods, client.InNamespace(sts.Namespace), client.MatchingLabels(podLabels)); err != nil {
		return nil, err
	}
	return pods, nil
}

func IsEnterprise(repo string) bool {
	path := strings.Split(repo, "/")
	if len(path) == 0 {
		return false
	}
	return strings.HasSuffix(path[len(path)-1], "-enterprise")
}

func IsPhoneHomeEnabled() bool {
	phEnabled, found := os.LookupEnv(n.PhoneHomeEnabledEnv)
	return !found || phEnabled == "true"
}

func GetOperatorVersion() string {
	return os.Getenv(n.OperatorVersionEnv)
}

func GetPardotID() string {
	return os.Getenv(n.PardotIDEnv)
}

func GetOperatorID(c *rest.Config) types.UID {
	uid, err := getOperatorDeploymentUID(c)
	if err == nil {
		return uid
	}

	return uuid.NewUUID()
}

func getOperatorDeploymentUID(c *rest.Config) (types.UID, error) {
	ns, okNS := os.LookupEnv(n.NamespaceEnv)
	podName, okPN := os.LookupEnv(n.PodNameEnv)

	if !okNS || !okPN {
		return "", fmt.Errorf("%s or %s is not defined", n.NamespaceEnv, n.PodNameEnv)
	}
	client, err := kubernetes.NewForConfig(c)
	if err != nil {
		return "", err
	}

	var d *appsv1.Deployment
	d, err = client.AppsV1().Deployments(ns).Get(context.TODO(), deploymentName(podName), metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return d.UID, nil
}

func deploymentName(podName string) string {
	s := strings.Split(podName, "-")
	return strings.Join(s[:len(s)-2], "-")
}
