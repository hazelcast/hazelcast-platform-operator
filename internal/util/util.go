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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

func CreateOrUpdateForce(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	opResult, err := controllerutil.CreateOrUpdate(ctx, c, obj, f)
	if kerrors.IsAlreadyExists(err) {
		// Ignore "already exists" error.
		// Inside createOrUpdate() there's is a race condition between Get() and Create(), so this error is expected from time to time.
		return opResult, nil
	}
	if kerrors.IsInvalid(err) {
		// try hard replace
		err := c.Delete(ctx, obj)
		if err != nil {
			return opResult, err
		}
		return controllerutil.CreateOrUpdate(ctx, c, obj, f)
	}
	return opResult, err
}

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	opResult, err := controllerutil.CreateOrUpdate(ctx, c, obj, f)
	if kerrors.IsAlreadyExists(err) {
		// Ignore "already exists" error.
		// Inside createOrUpdate() there's is a race condition between Get() and Create(), so this error is expected from time to time.
		return opResult, nil
	}
	return opResult, err
}

func Update(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {
	key := client.ObjectKeyFromObject(obj)
	existing := obj.DeepCopyObject() //nolint
	if err := mutate(f, key, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	if equality.Semantic.DeepEqual(existing, obj) {
		return controllerutil.OperationResultNone, nil
	}

	if err := c.Update(ctx, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

// mutate wraps a MutateFn and applies validation to its result.
func mutate(f controllerutil.MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
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

func AddFinalizer(ctx context.Context, c client.Client, object client.Object, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(object, n.Finalizer) && object.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(object, n.Finalizer)
		err := c.Update(ctx, object)
		if err != nil {
			return err
		}
		logger.V(DebugLevel).Info("Finalizer added into custom resource successfully")
	}
	return nil
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
	if err == nil {
		return nil, false
	}
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

type ExternalAddresser interface {
	metav1.Object
	ExternalAddressEnabled() bool
}

func GetExternalAddresses(
	ctx context.Context,
	cli client.Client,
	cr ExternalAddresser,
	logger logr.Logger,
) ([]string, []string) {
	svcList, err := getRelatedServices(ctx, cli, cr)
	if err != nil {
		logger.Error(err, "Could not get the service")
		return nil, nil
	}

	var externalAddrs, wanAddrs []string
	for i := range svcList.Items {
		svc := svcList.Items[i]
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			addr := getLoadBalancerAddress(&ingress)
			if addr == "" {
				continue
			}
			for _, port := range svc.Spec.Ports {
				// we don't want to print these ports as the output of "kubectl get hz" command
				// and we want to print wan addresses with a separate title (WAN-Addresses)
				if strings.HasPrefix(port.Name, n.WanPortNamePrefix) {
					wanAddrs = append(wanAddrs, fmt.Sprintf("%s:%d", addr, port.Port))
					continue
				}
				if port.Port == int32(n.RestServerSocketPort) {
					continue
				}
				if port.Port == int32(n.MemberServerSocketPort) {
					continue
				}
				externalAddrs = append(externalAddrs, fmt.Sprintf("%s:%d", addr, port.Port))
			}
		}

		if len(externalAddrs) == 0 {
			logger.Info("Load Balancer external IP is not ready.")
		}
	}

	return externalAddrs, wanAddrs
}

func getRelatedServices(ctx context.Context, cli client.Client, cr ExternalAddresser) (*corev1.ServiceList, error) {
	nsMatcher := client.InNamespace(cr.GetNamespace())
	labelMatcher := client.MatchingLabels(labels(cr))

	var svcList corev1.ServiceList
	if err := cli.List(ctx, &svcList, nsMatcher, labelMatcher); err != nil {
		return nil, err
	}

	return &svcList, nil
}

func labels(cr ExternalAddresser) map[string]string {
	return map[string]string{
		n.ApplicationNameLabel:         n.Hazelcast,
		n.ApplicationInstanceNameLabel: cr.GetName(),
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
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

func DeleteObject(ctx context.Context, c client.Client, obj client.Object) error {
	err := c.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}
	return nil
}

func IsApplied(obj client.Object) bool {
	_, ok := obj.GetAnnotations()[n.LastAppliedSpecAnnotation]
	return ok
}

func IsSuccessfullyApplied(obj client.Object) bool {
	_, ok := obj.GetAnnotations()[n.LastSuccessfulSpecAnnotation]
	return ok
}

func NodeDiscoveryEnabled() bool {
	watching, found := os.LookupEnv(n.HazelcastNodeDiscoveryEnabledEnv)
	if !found {
		return true
	}
	return watching == "true"
}

func EnrichServiceNodePorts(newPorts []v1.ServicePort, existing []v1.ServicePort) []v1.ServicePort {
	existingMap := map[string]v1.ServicePort{}
	for _, port := range existing {
		existingMap[port.Name] = port
	}

	for i := range newPorts {
		if val, ok := existingMap[newPorts[i].Name]; ok {
			newPorts[i].NodePort = val.NodePort
		}
	}
	return newPorts
}
