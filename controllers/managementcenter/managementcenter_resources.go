package managementcenter

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-enterprise-operator/controllers/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ManagementCenterReconciler) reconcileStatefulset(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	ls := labels(mc)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metadata(mc),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: "management-center",
						Ports: []v1.ContainerPort{{
							ContainerPort: 8080,
							Name:          "mancenter",
							Protocol:      v1.ProtocolTCP,
						}},
					}},
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(mc, sts, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Statefulset")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, sts, func() error {
		// Management Center StatefulSet size is always 1
		sts.Spec.Replicas = &[]int32{1}[0]
		sts.Spec.Template.Spec.Containers[0].Image = dockerImage(mc)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", mc.Name, "result", opResult)
	}
	return err
}

func labels(mc *hazelcastv1alpha1.ManagementCenter) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "management-center",
		"app.kubernetes.io/instance":   mc.Name,
		"app.kubernetes.io/managed-by": "hazelcast-enterprise-operator",
	}
}

func metadata(mc *hazelcastv1alpha1.ManagementCenter) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mc.Name,
		Namespace: mc.Namespace,
		Labels:    labels(mc),
	}
}

func dockerImage(mc *hazelcastv1alpha1.ManagementCenter) string {
	return fmt.Sprintf("%s:%s", mc.Spec.Repository, mc.Spec.Version)
}

func (r *ManagementCenterReconciler) reconcileService(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	service := &corev1.Service{
		ObjectMeta: metadata(mc),
		Spec: corev1.ServiceSpec{
			Selector: labels(mc),
			Ports:    ports(),
		},
	}

	err := controllerutil.SetControllerReference(mc, service, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Service")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Type = mc.Spec.ExternalConnectivity.Type.ManagementCenterServiceType()
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", mc.Name, "result", opResult)
	}
	return err
}

func ports() []v1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			Port:       8080,
			TargetPort: intstr.FromString("mancenter"),
		},
		{
			Name:       "https",
			Protocol:   corev1.ProtocolTCP,
			Port:       443,
			TargetPort: intstr.FromString("mancenter"),
		},
	}
}
