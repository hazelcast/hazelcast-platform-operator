package controllers

import (
	"fmt"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *HazelcastReconciler) statefulSetForHazelcast(h *hazelcastv1alpha1.Hazelcast) (*appsv1.StatefulSet, error) {
	ls := labelsForHazelcast(h)
	replicas := h.Spec.ClusterSize

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.StatefulSetSpec{
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
						Image: ImageForCluster(h),
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
		},
	}
	// Set Hazelcast instance as the owner and controller
	err := ctrl.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		return nil, err
	}

	return sts, nil
}

func (r *HazelcastReconciler) serviceAccountForDiscovery(h *hazelcastv1alpha1.Hazelcast) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labelsForHazelcast(h),
		},
	}

	err := ctrl.SetControllerReference(h, serviceAccount, r.Scheme)
	if err != nil {
		return nil, err
	}
	return serviceAccount, nil
}

func (r *HazelcastReconciler) clusterRoleForDiscovery(h *hazelcastv1alpha1.Hazelcast) (*rbacv1.ClusterRole, error) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labelsForHazelcast(h),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "nodes", "services"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	err := ctrl.SetControllerReference(h, clusterRole, r.Scheme)
	if err != nil {
		return nil, err
	}
	return clusterRole, nil
}

func (r *HazelcastReconciler) clusterRoleBindingForDiscovery(h *hazelcastv1alpha1.Hazelcast) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labelsForHazelcast(h),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      h.Name,
				Namespace: h.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     h.Name,
		},
	}

	err := ctrl.SetControllerReference(h, clusterRoleBinding, r.Scheme)
	if err != nil {
		return nil, err
	}
	return clusterRoleBinding, nil
}

func labelsForHazelcast(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "hazelcast",
		"app.kubernetes.io/instance":   h.Name,
		"app.kubernetes.io/managed-by": "hazelcast-enterprise-operator",
	}
}

func ImageForCluster(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s:%s", h.Spec.Repository, h.Spec.Version)
}
