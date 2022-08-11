package util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	opResult, err := controllerutil.CreateOrUpdate(ctx, c, obj, f)
	if kerrors.IsAlreadyExists(err) {
		// Ignore "already exists" error.
		// Inside createOrUpdate() there's is a race condition between Get() and Create(), so this error is expected from time to time.
		return opResult, nil
	}
	return opResult, err
}

func CreateOrGet(ctx context.Context, c client.Client, key client.ObjectKey, obj client.Object) error {
	err := c.Get(ctx, key, obj)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		} else {
			return c.Create(ctx, obj)
		}
	} else {
		return nil
	}
}

func CheckIfRunning(ctx context.Context, cl client.Client, namespacedName types.NamespacedName, expectedReplicas int32) (bool, error) {
	sts := &appsv1.StatefulSet{}
	err := cl.Get(ctx, client.ObjectKey{Name: namespacedName.Name, Namespace: namespacedName.Namespace}, sts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
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
		} else if hasPodFailedWhileWaiting(&pod) {
			errs = append(errs, errorsFromPendingPod(&pod)...)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// AsPodErrors tries to transform err to PodErrors and return it with true.
// If it is not possible nil and false is returned.
func AsPodErrors(err error) (PodErrors, bool) {
	t := new(PodErrors)
	if errors.As(err, t) {
		return *t, true
	}
	return nil, false
}

func hasPodFailedWhileWaiting(pod *corev1.Pod) bool {
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
				podErrors = append(podErrors, NewPodErrorWithContainerStatus(pod, status))
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

func IsDeveloperModeEnabled() bool {
	value := os.Getenv(n.DeveloperModeEnabledEnv)
	return strings.ToLower(value) == "true"
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

type ExternalAddresser interface {
	metav1.Object
	ExternalAddressEnabled() bool
}

func GetExternalAddresses(
	ctx context.Context,
	cli client.Client,
	cr ExternalAddresser,
	logger logr.Logger,
) string {
	if !cr.ExternalAddressEnabled() {
		return ""
	}

	svc, err := getDiscoveryService(ctx, cli, cr)
	if err != nil {
		logger.Error(err, "Could not get the service")
		return ""
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		logger.Error(errors.New("unexpected service type"), "Service type is not LoadBalancer")
		return ""
	}

	externalAddrs := make([]string, 0, len(svc.Status.LoadBalancer.Ingress)*len(svc.Spec.Ports))
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		addr := getLoadBalancerAddress(&ingress)
		if addr == "" {
			continue
		}
		for _, port := range svc.Spec.Ports {
			externalAddrs = append(externalAddrs, fmt.Sprintf("%s:%d", addr, port.Port))
		}
	}

	result := strings.Join(externalAddrs, ",")
	if result == "" {
		logger.Info("LoadBalancer external IP is not ready")
		return ""
	}
	return result
}

func getDiscoveryService(ctx context.Context, cli client.Client, cr ExternalAddresser) (*corev1.Service, error) {
	svc := corev1.Service{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, &svc); err != nil {
		return nil, err
	}
	return &svc, nil
}

func getLoadBalancerAddress(lb *corev1.LoadBalancerIngress) string {
	if lb.IP != "" {
		return lb.IP
	}
	if lb.Hostname != "" {
		return lb.Hostname
	}
	return ""
}

func IndexConfigSliceEquals(a, b []hazelcastv1alpha1.IndexConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if !indexConfigEquals(v, b[i]) {
			return false
		}
	}
	return true
}

func indexConfigEquals(a, b hazelcastv1alpha1.IndexConfig) bool {
	if a.Name != b.Name {
		return false
	}

	if a.Type != b.Type {
		return false
	}

	if !stringSliceEquals(a.Attributes, b.Attributes) {
		return false
	}

	if a.BitmapIndexOptions != b.BitmapIndexOptions {
		return false
	}
	return true
}

func stringSliceEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
