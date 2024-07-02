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
		// Inside createOrUpdate() there's a race condition between Get() and Create(), so this error is expected from time to time.
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
		// Inside createOrUpdate() there's a race condition between Get() and Create(), so this error is expected from time to time.
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

func CheckIfRunning(ctx context.Context, cl client.Client, statefulSet *appsv1.StatefulSet, expectedReplicas int32) (bool, error) {
	if isStatefulSetReady(statefulSet, expectedReplicas) {
		return true, nil
	}
	if err := checkPodsForFailure(ctx, cl, statefulSet); err != nil {
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

func ListRelatedServices(ctx context.Context, cli client.Client, cr ExternalAddresser) (*corev1.ServiceList, error) {
	nsMatcher := client.InNamespace(cr.GetNamespace())
	labelMatcher := client.MatchingLabels(Labels(cr))

	var svcList corev1.ServiceList
	if err := cli.List(ctx, &svcList, nsMatcher, labelMatcher); err != nil {
		return nil, err
	}

	return &svcList, nil
}

func Labels(cr ExternalAddresser) map[string]string {
	return map[string]string{
		n.ApplicationNameLabel:         n.Hazelcast,
		n.ApplicationInstanceNameLabel: cr.GetName(),
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
}

func GetExternalAddressesForMC(
	ctx context.Context,
	cli client.Client,
	cr ExternalAddresser,
	logger logr.Logger,
) []string {
	if !cr.ExternalAddressEnabled() {
		return nil
	}

	svc, err := getDiscoveryService(ctx, cli, cr)
	if err != nil {
		logger.Error(err, "Could not get the service")
		return nil
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		logger.Error(errors.New("unexpected service type"), "Service type is not LoadBalancer")
		return nil
	}

	externalAddrs := make([]string, 0, len(svc.Status.LoadBalancer.Ingress)*len(svc.Spec.Ports))
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		addr := GetLoadBalancerAddress(&ingress)
		if addr == "" {
			continue
		}
		for _, port := range svc.Spec.Ports {
			externalAddrs = append(externalAddrs, fmt.Sprintf("%s:%d", addr, port.Port))
		}
	}

	if len(externalAddrs) == 0 {
		logger.Info("Load Balancer external IP is not ready.")
	}
	return externalAddrs
}

func getDiscoveryService(ctx context.Context, cli client.Client, cr ExternalAddresser) (*corev1.Service, error) {
	svc := corev1.Service{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: cr.GetNamespace(), Name: cr.GetName()}, &svc); err != nil {
		return nil, err
	}
	return &svc, nil
}

func GetExternalAddress(svc *corev1.Service) string {
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		addr := GetLoadBalancerAddress(&ingress)
		if addr != "" {
			return addr
		}
	}
	return ""
}

func GetLoadBalancerAddress(lb *corev1.LoadBalancerIngress) string {
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

func NodeDiscoveryEnabled() bool {
	watching, found := os.LookupEnv(n.HazelcastNodeDiscoveryEnabledEnv)
	if !found {
		return true
	}
	return watching == "true"
}

func EnrichServiceNodePorts(desired []corev1.ServicePort, current []corev1.ServicePort) []corev1.ServicePort {
	existingMap := map[string]corev1.ServicePort{}
	for _, port := range current {
		existingMap[port.Name] = port
	}

	for i := range desired {
		if val, ok := existingMap[desired[i].Name]; ok {
			desired[i].NodePort = val.NodePort
		}
	}

	return desired
}
