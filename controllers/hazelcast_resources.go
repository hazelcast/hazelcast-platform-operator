package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizer = "hazelcast.com/finalizer"

func (r *HazelcastReconciler) addFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(h, finalizer) {
		controllerutil.AddFinalizer(h, finalizer)
		err := r.Update(ctx, h)
		if err != nil {
			logger.Error(err, "Failed to add finalizer into custom resource")
			return err
		}
		logger.V(1).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *HazelcastReconciler) executeFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if err := r.removeClusterRole(ctx, h, logger); err != nil {
		logger.Error(err, "ClusterRole could not be removed")
		return err
	}
	controllerutil.RemoveFinalizer(h, finalizer)
	err := r.Update(ctx, h)
	if err != nil {
		logger.Error(err, "Failed to remove finalizer from custom resource")
		return err
	}
	return nil
}

func (r *HazelcastReconciler) reconcileClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   h.Name,
			Labels: labelsForHazelcast(h),
		},
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "nodes", "services"},
				Verbs:     []string{"get", "list"},
			},
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ClusterRole", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileServiceAccount(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labelsForHazelcast(h),
		},
	}

	err := controllerutil.SetControllerReference(h, serviceAccount, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on ServiceAccount")
		return err
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, serviceAccount, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ServiceAccount", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labelsForHazelcast(h),
		},
	}

	err := controllerutil.SetControllerReference(h, roleBinding, r.Scheme)
	if err != nil {
		return err
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, roleBinding, func() error {
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      h.Name,
				Namespace: h.Namespace,
			},
		}
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     h.Name,
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "RoleBinding", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileStatefulset(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labelsForHazelcast(h),
		},
	}

	err := controllerutil.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Statefulset")
		return err
	}

	opResult, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		ls := labelsForHazelcast(h)
		replicas := h.Spec.ClusterSize
		sts.Spec = appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image: imageForCluster(h),
						Name:  "hazelcast",
						Ports: []v1.ContainerPort{{
							ContainerPort: 5701,
							Name:          "hazelcast",
						}},
						Env: []v1.EnvVar{
							{
								Name:  "HZ_NETWORK_JOIN_KUBERNETES_ENABLED",
								Value: "true",
							},
							{
								Name:  "HZ_NETWORK_JOIN_KUBERNETES_PODLABELNAME",
								Value: "app.kubernetes.io/instance",
							},
							{
								Name:  "HZ_NETWORK_JOIN_KUBERNETES_PODLABELVALUE",
								Value: h.Name,
							},
						},
					}},
					ServiceAccountName: h.Name,
				},
			},
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) removeClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	clusterRole := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: h.Name}, clusterRole)
	if err != nil && errors.IsNotFound(err) {
		logger.V(1).Info("ClusterRole is not created yet. Or it is already removed.")
		return nil
	}

	err = r.Delete(ctx, clusterRole)
	if err != nil {
		logger.Error(err, "Failed to clean up ClusterRole")
		return err
	}
	logger.V(1).Info("ClusterRole removed successfully")
	return nil
}

func labelsForHazelcast(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "hazelcast",
		"app.kubernetes.io/instance":   h.Name,
		"app.kubernetes.io/managed-by": "hazelcast-enterprise-operator",
	}
}

func imageForCluster(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s:%s", h.Spec.Repository, h.Spec.Version)
}
