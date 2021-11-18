package managementcenter

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/platform"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Environment variables used for Management Center configuration
const (
	// mcLicenseKey License key for Management Center
	mcLicenseKey = "MC_LICENSE_KEY"
	// mcInitCmd init command for Management Center
	mcInitCmd = "MC_INIT_CMD"
	javaOpts  = "JAVA_OPTS"
)

func (r *ManagementCenterReconciler) reconcileRole(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	pt, err := platform.GetType()
	if err != nil {
		return err
	}

	if pt == platform.Kubernetes {
		return nil
	}

	role := &rbacv1.Role{
		ObjectMeta: metadata(mc),
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"security.openshift.io"},
				Resources: []string{"securitycontextconstraints"},
				Verbs:     []string{"use"},
			},
		},
	}

	err = controllerutil.SetControllerReference(mc, role, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Role")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, role, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Role", mc.Name, "result", opResult)
	}
	return err
}

func (r *ManagementCenterReconciler) reconcileServiceAccount(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	pt, err := platform.GetType()
	if err != nil {
		return err
	}

	if pt == platform.Kubernetes {
		return nil
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metadata(mc),
	}

	err = controllerutil.SetControllerReference(mc, serviceAccount, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on ServiceAccount")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, serviceAccount, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ServiceAccount", mc.Name, "result", opResult)
	}
	return err
}

func (r *ManagementCenterReconciler) reconcileRoleBinding(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	pt, err := platform.GetType()
	if err != nil {
		return err
	}

	if pt == platform.Kubernetes {
		return nil
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metadata(mc),
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      mc.Name,
				Namespace: mc.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     mc.Name,
		},
	}
	err = controllerutil.SetControllerReference(mc, rb, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on RoleBinding")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, rb, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "RoleBinding", mc.Name, "result", opResult)
	}
	return err
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
		service.Spec.Type = mc.Spec.ExternalConnectivity.ManagementCenterServiceType()
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", mc.Name, "result", opResult)
	}
	return err
}

func metadata(mc *hazelcastv1alpha1.ManagementCenter) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      mc.Name,
		Namespace: mc.Namespace,
		Labels:    labels(mc),
	}
}
func labels(mc *hazelcastv1alpha1.ManagementCenter) map[string]string {
	return map[string]string{
		n.ApplicationNameLabel:         n.ManagementCenter,
		n.ApplicationInstanceNameLabel: mc.Name,
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
}

func ports() []v1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			Port:       8080,
			TargetPort: intstr.FromString(n.Mancenter),
		},
		{
			Name:       "https",
			Protocol:   corev1.ProtocolTCP,
			Port:       443,
			TargetPort: intstr.FromString(n.Mancenter),
		},
	}
}

func (r *ManagementCenterReconciler) reconcileStatefulset(ctx context.Context, mc *hazelcastv1alpha1.ManagementCenter, logger logr.Logger) error {
	ls := labels(mc)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metadata(mc),
		Spec: appsv1.StatefulSetSpec{
			// Management Center StatefulSet size is always 1
			Replicas: &[]int32{1}[0],
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name: n.ManagementCenter,
						Ports: []v1.ContainerPort{{
							ContainerPort: 8080,
							Name:          n.Mancenter,
							Protocol:      v1.ProtocolTCP,
						}},
						VolumeMounts: []corev1.VolumeMount{},
						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/health",
									Port:   intstr.FromInt(8081),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/health",
									Port:   intstr.FromInt(8081),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						SecurityContext: &v1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{65534}[0],
							Privileged:               &[]bool{false}[0],
							ReadOnlyRootFilesystem:   &[]bool{false}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &v1.Capabilities{
								Drop: []v1.Capability{"ALL"},
							},
						},
					}},
					SecurityContext: &v1.PodSecurityContext{
						FSGroup: &[]int64{65534}[0],
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{},
		},
	}

	pt, err := platform.GetType()
	if err != nil {
		return err
	}

	if pt == platform.OpenShift {
		sts.Spec.Template.Spec.ServiceAccountName = mc.Name
	}

	err = controllerutil.SetControllerReference(mc, sts, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Statefulset")
		return err
	}

	if mc.Spec.Persistence.IsEnabled() {
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{persistentVolumeMount()}
		sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, persistentVolumeClaim(mc))
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Spec.Template.Spec.Containers[0].Image = mc.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env = env(mc)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", mc.Name, "result", opResult)
	}
	return err
}

func persistentVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      n.MancenterStorageName,
		MountPath: "/data",
	}
}

func persistentVolumeClaim(mc *hazelcastv1alpha1.ManagementCenter) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.MancenterStorageName,
			Namespace: mc.Namespace,
			Labels:    labels(mc),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: mc.Spec.Persistence.StorageClass,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mc.Spec.Persistence.Size,
				},
			},
		},
	}
}

func env(mc *hazelcastv1alpha1.ManagementCenter) []v1.EnvVar {
	envs := []v1.EnvVar{{Name: mcInitCmd, Value: clusterAddCommand(mc)}}

	if mc.Spec.LicenseKeySecret != "" {
		envs = append(envs,
			v1.EnvVar{
				Name: mcLicenseKey,
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: mc.Spec.LicenseKeySecret,
						},
						Key: n.LicenseDataKey,
					},
				},
			},
			v1.EnvVar{
				Name:  javaOpts,
				Value: "-Dhazelcast.mc.license=$(MC_LICENSE_KEY) -Dhazelcast.mc.healthCheck.enable=true -Dhazelcast.mc.tls.enabled=false -Dmancenter.ssl=false",
			},
		)
	} else {
		envs = append(envs,
			v1.EnvVar{
				Name:  javaOpts,
				Value: "-Dhazelcast.mc.healthCheck.enable=true -Dhazelcast.mc.tls.enabled=false -Dmancenter.ssl=false",
			},
		)
	}
	return envs
}

func clusterAddCommand(mc *hazelcastv1alpha1.ManagementCenter) string {
	clusters := mc.Spec.HazelcastClusters
	strs := make([]string, len(clusters))
	for i, cluster := range clusters {
		strs[i] = fmt.Sprintf("./bin/mc-conf.sh cluster add --lenient=true -H /data -cn %s -ma %s", cluster.Name, cluster.Address)
	}
	return strings.Join(strs, " && ")
}
