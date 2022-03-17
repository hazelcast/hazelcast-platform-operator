package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"strconv"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/config"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/platform"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/util"
)

// Environment variables used for Hazelcast cluster configuration
const (
	// hzLicenseKey License key for Hazelcast cluster
	hzLicenseKey = "HZ_LICENSEKEY"
)

func (r *HazelcastReconciler) addFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(h, n.Finalizer) && h.GetDeletionTimestamp() == nil {
		controllerutil.AddFinalizer(h, n.Finalizer)
		err := r.Update(ctx, h)
		if err != nil {
			return err
		}
		logger.V(1).Info("Finalizer added into custom resource successfully")
	}
	return nil
}

func (r *HazelcastReconciler) executeFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(h, n.Finalizer) {
		return nil
	}

	if err := r.removeClusterRole(ctx, h, logger); err != nil {
		logger.Error(err, "ClusterRole could not be removed")
		return err
	}
	if err := r.removeClusterRoleBinding(ctx, h, logger); err != nil {
		logger.Error(err, "ClusterRoleBinding could not be removed")
		return err
	}
	controllerutil.RemoveFinalizer(h, n.Finalizer)
	err := r.Update(ctx, h)
	if err != nil {
		logger.Error(err, "Failed to remove finalizer from custom resource")
		return err
	}
	if util.IsPhoneHomeEnabled() {
		delete(r.metrics.HazelcastMetrics, h.UID)
	}
	key := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	if c, ok := r.hzClients.Load(key); ok {
		r.hzClients.Delete(key)
		c.(*HazelcastClient).shutdown(ctx)
	}
	return nil
}

func (r *HazelcastReconciler) removeClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	clusterRole := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: h.ClusterScopedName()}, clusterRole)
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

func (r *HazelcastReconciler) removeClusterRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	crb := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: h.ClusterScopedName()}, crb)
	if err != nil && errors.IsNotFound(err) {
		logger.V(1).Info("ClusterRoleBinding is not created yet. Or it is already removed.")
		return nil
	}

	err = r.Delete(ctx, crb)
	if err != nil {
		logger.Error(err, "Failed to clean up ClusterRoleBinding")
		return err
	}
	logger.V(1).Info("ClusterRoleBinding removed successfully")
	return nil
}

func (r *HazelcastReconciler) reconcileClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   h.ClusterScopedName(),
			Labels: labels(h),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "nodes", "services"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	if platform.GetType() == platform.OpenShift {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"security.openshift.io"},
			Resources: []string{"securitycontextconstraints"},
			Verbs:     []string{"use"},
		},
		)
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, clusterRole, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ClusterRole", h.ClusterScopedName(), "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileServiceAccount(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metadata(h),
	}

	err := controllerutil.SetControllerReference(h, serviceAccount, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on ServiceAccount")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, serviceAccount, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ServiceAccount", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileClusterRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	csName := h.ClusterScopedName()
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   csName,
			Labels: labels(h),
		},
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, crb, func() error {
		crb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      h.Name,
				Namespace: h.Namespace,
			},
		}
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     csName,
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ClusterRoleBinding", csName, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileService(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	service := &corev1.Service{
		ObjectMeta: metadata(h),
		Spec: corev1.ServiceSpec{
			Selector: labels(h),
			Ports:    ports(),
		},
	}

	err := controllerutil.SetControllerReference(h, service, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Service")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec.Type = serviceType(h)
		if serviceType(h) == corev1.ServiceTypeClusterIP {
			// dirty hack to prevent the error when changing the service type
			service.Spec.Ports[0].NodePort = 0
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", h.Name, "result", opResult)
	}
	return err
}

func serviceType(h *hazelcastv1alpha1.Hazelcast) v1.ServiceType {
	if h.Spec.ExposeExternally.IsEnabled() {
		return h.Spec.ExposeExternally.DiscoveryK8ServiceType()
	}
	return corev1.ServiceTypeClusterIP
}

func (r *HazelcastReconciler) reconcileServicePerPod(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !h.Spec.ExposeExternally.IsSmart() {
		// Service per pod applies only to Smart type
		return nil
	}

	for i := 0; i < int(h.Spec.ClusterSize); i++ {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      servicePerPodName(i, h),
				Namespace: h.Namespace,
				Labels:    servicePerPodLabels(h),
			},
			Spec: corev1.ServiceSpec{
				Selector:                 servicePerPodSelector(i, h),
				Ports:                    ports(),
				PublishNotReadyAddresses: true,
			},
		}

		err := controllerutil.SetControllerReference(h, service, r.Scheme)
		if err != nil {
			return err
		}

		opResult, err := util.CreateOrUpdate(ctx, r.Client, service, func() error {
			service.Spec.Type = h.Spec.ExposeExternally.MemberAccessServiceType()
			return nil
		})

		if opResult != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Service", servicePerPodName(i, h), "result", opResult)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *HazelcastReconciler) reconcileUnusedServicePerPod(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {
	var s int
	if h.Spec.ExposeExternally.IsSmart() {
		s = int(h.Spec.ClusterSize)
	}

	// Delete unused services (when the cluster was scaled down)
	// The current number of service per pod is always stored in the StatefulSet annotations
	sts := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: h.Name, Namespace: h.Namespace}, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			// Not found, StatefulSet is not created yet, no need to delete any services
			return nil
		}
		return err
	}
	p, err := strconv.Atoi(sts.ObjectMeta.Annotations[n.ServicePerPodCountAnnotation])
	if err != nil {
		// Annotation not found, no need to delete any services
		return nil
	}

	for i := s; i < p; i++ {
		s := &v1.Service{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: servicePerPodName(i, h), Namespace: h.Namespace}, s)
		if err != nil {
			if errors.IsNotFound(err) {
				// Not found, no need to remove the service
				continue
			}
			return err
		}
		err = r.Client.Delete(ctx, s)
		if err != nil {
			if errors.IsNotFound(err) {
				// Not found, no need to remove the service
				continue
			}
			return err
		}
	}

	return nil
}

func servicePerPodName(i int, h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s-%d", h.Name, i)
}

func servicePerPodSelector(i int, h *hazelcastv1alpha1.Hazelcast) map[string]string {
	ls := labels(h)
	ls[n.PodNameLabel] = servicePerPodName(i, h)
	return ls
}

func servicePerPodLabels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	ls := labels(h)
	ls[n.ServicePerPodLabelName] = n.LabelValueTrue
	return ls
}

func ports() []v1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       n.HazelcastPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       n.DefaultHzPort,
			TargetPort: intstr.FromString(n.Hazelcast),
		},
	}
}

func (r *HazelcastReconciler) isServicePerPodReady(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, _ logr.Logger) bool {
	if !h.Spec.ExposeExternally.IsSmart() {
		// Service per pod applies only to Smart type
		return true
	}

	// Check if each service per pod is ready
	for i := 0; i < int(h.Spec.ClusterSize); i++ {
		s := &v1.Service{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: servicePerPodName(i, h), Namespace: h.Namespace}, s)
		if err != nil {
			// Service is not created yet
			return false
		}
		if s.Spec.Type == v1.ServiceTypeLoadBalancer {
			if len(s.Status.LoadBalancer.Ingress) == 0 {
				// LoadBalancer service waiting for External IP to get assigned
				return false
			}
		}
	}

	return true
}

func (r *HazelcastReconciler) reconcileConfigMap(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metadata(h),
	}

	err := controllerutil.SetControllerReference(h, cm, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on ConfigMap")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data, err = hazelcastConfigMapData(h)
		return err
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ConfigMap", h.Name, "result", opResult)
	}
	return err
}

func hazelcastConfigMapData(h *hazelcastv1alpha1.Hazelcast) (map[string]string, error) {
	cfg := hazelcastConfigMapStruct(h)
	yml, err := yaml.Marshal(config.HazelcastWrapper{Hazelcast: cfg})
	if err != nil {
		return nil, err
	}
	return map[string]string{"hazelcast.yaml": string(yml)}, nil
}

func hazelcastConfigMapStruct(h *hazelcastv1alpha1.Hazelcast) config.Hazelcast {
	cfg := config.Hazelcast{
		Jet: config.Jet{
			Enabled: &[]bool{true}[0],
		},
		Network: config.Network{
			Join: config.Join{
				Kubernetes: config.Kubernetes{
					Enabled:     &[]bool{true}[0],
					ServiceName: h.Name,
				},
			},
			RestAPI: config.RestAPI{
				Enabled: &[]bool{true}[0],
				EndpointGroups: config.EndpointGroups{
					HealthCheck: config.EndpointGroup{
						Enabled: &[]bool{true}[0],
					},
					ClusterWrite: config.EndpointGroup{
						Enabled: &[]bool{true}[0],
					},
					Persistence: config.EndpointGroup{
						Enabled: &[]bool{true}[0],
					},
				},
			},
		},
	}

	if h.Spec.ExposeExternally.UsesNodeName() {
		cfg.Network.Join.Kubernetes.UseNodeNameAsExternalAddress = &[]bool{true}[0]
	}

	if h.Spec.ExposeExternally.IsSmart() {
		cfg.Network.Join.Kubernetes.ServicePerPodLabelName = n.ServicePerPodLabelName
		cfg.Network.Join.Kubernetes.ServicePerPodLabelValue = n.LabelValueTrue
	}

	if h.Spec.ClusterName != "" {
		cfg.ClusterName = h.Spec.ClusterName
	}

	if h.Spec.Persistence.IsEnabled() {
		cfg.Persistence = config.Persistence{
			Enabled:                   &[]bool{true}[0],
			BaseDir:                   h.Spec.Persistence.BaseDir,
			BackupDir:                 h.Spec.Persistence.BaseDir + "/hot-backup",
			Parallelism:               1,
			ValidationTimeoutSec:      120,
			DataLoadTimeoutSec:        900,
			ClusterDataRecoveryPolicy: clusterDataRecoveryPolicy(h.Spec.Persistence.ClusterDataRecoveryPolicy),
			AutoRemoveStaleData:       &[]bool{h.Spec.Persistence.AutoRemoveStaleData()}[0],
		}
	}
	return cfg
}

func (r *HazelcastReconciler) reconcileStatefulset(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	ls := labels(h)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metadata(h),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			ServiceName: h.Name,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: v1.PodSpec{
					ServiceAccountName:        h.Name,
					Affinity:                  &h.Spec.Scheduling.Affinity,
					Tolerations:               h.Spec.Scheduling.Tolerations,
					NodeSelector:              h.Spec.Scheduling.NodeSelector,
					TopologySpreadConstraints: h.Spec.Scheduling.TopologySpreadConstraints,
					SecurityContext: &v1.PodSecurityContext{
						FSGroup:      &[]int64{65534}[0],
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{65534}[0],
					},
					Containers: []v1.Container{{
						Name: n.Hazelcast,
						Ports: []v1.ContainerPort{{
							ContainerPort: n.DefaultHzPort,
							Name:          n.Hazelcast,
							Protocol:      v1.ProtocolTCP,
						}},
						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/hazelcast/health/node-state",
									Port:   intstr.FromInt(n.DefaultHzPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 0,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/hazelcast/health/node-state",
									Port:   intstr.FromInt(n.DefaultHzPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 0,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						SecurityContext: &v1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{65534}[0],
							Privileged:               &[]bool{false}[0],
							ReadOnlyRootFilesystem:   &[]bool{!h.Spec.Persistence.IsEnabled()}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &v1.Capabilities{
								Drop: []v1.Capability{"ALL"},
							},
						},
						VolumeMounts: volumeMount(h),
					}},
					TerminationGracePeriodSeconds: &[]int64{600}[0],
					Volumes:                       volumes(h),
				},
			},
		},
	}

	if h.Spec.Persistence.IsEnabled() {
		if h.Spec.Persistence.UseHostPath() {
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, hostPathVolume(h))
			sts.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot = &[]bool{false}[0]
			sts.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = &[]int64{0}[0]
		} else {
			sts.Spec.VolumeClaimTemplates = persistentVolumeClaim(h)
		}
	}

	err := controllerutil.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on Statefulset")
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Spec.Replicas = &h.Spec.ClusterSize
		sts.ObjectMeta.Annotations = statefulSetAnnotations(h)
		sts.Spec.Template.Annotations, err = podAnnotations(h)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.ImagePullSecrets = h.Spec.ImagePullSecrets
		sts.Spec.Template.Spec.Containers[0].Image = h.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env = env(h)
		sts.Spec.Template.Spec.Containers[0].ImagePullPolicy = h.Spec.ImagePullPolicy
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", h.Name, "result", opResult)
	}
	return err
}

func volumes(h *hazelcastv1alpha1.Hazelcast) []v1.Volume {
	return []v1.Volume{
		{
			Name: n.HazelcastStorageName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: h.Name,
					},
				},
			},
		},
	}
}

func hostPathVolume(h *hazelcastv1alpha1.Hazelcast) v1.Volume {
	return v1.Volume{
		Name: n.PersistenceVolumeName,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Path: h.Spec.Persistence.HostPath,
				Type: &[]v1.HostPathType{v1.HostPathDirectoryOrCreate}[0],
			},
		},
	}
}

func persistentVolumeClaim(h *hazelcastv1alpha1.Hazelcast) []v1.PersistentVolumeClaim {
	return []v1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n.PersistenceVolumeName,
				Namespace: h.Namespace,
				Labels:    labels(h),
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: h.Spec.Persistence.Pvc.AccessModes,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						corev1.ResourceStorage: h.Spec.Persistence.Pvc.RequestStorage,
					},
				},
				StorageClassName: h.Spec.Persistence.Pvc.StorageClassName,
			},
		},
	}
}

func volumeMount(h *hazelcastv1alpha1.Hazelcast) []corev1.VolumeMount {
	mounts := []v1.VolumeMount{
		{
			Name:      n.HazelcastStorageName,
			MountPath: n.HazelcastMountPath,
		},
	}
	if h.Spec.Persistence.IsEnabled() {
		mounts = append(mounts, v1.VolumeMount{
			Name:      n.PersistenceVolumeName,
			MountPath: h.Spec.Persistence.BaseDir,
		})
	}
	return mounts
}

func clusterDataRecoveryPolicy(policyType hazelcastv1alpha1.DataRecoveryPolicyType) string {
	switch policyType {
	case hazelcastv1alpha1.FullRecovery:
		return "FULL_RECOVERY_ONLY"
	case hazelcastv1alpha1.MostRecent:
		return "PARTIAL_RECOVERY_MOST_RECENT"
	case hazelcastv1alpha1.MostComplete:
		return "PARTIAL_RECOVERY_MOST_COMPLETE"
	}
	return "FULL_RECOVERY_ONLY"
}

func env(h *hazelcastv1alpha1.Hazelcast) []v1.EnvVar {
	envs := []v1.EnvVar{
		{
			Name:  "JAVA_OPTS",
			Value: fmt.Sprintf("-Dhazelcast.config=%s/hazelcast.yaml", n.HazelcastMountPath),
		},
		{
			Name:  "HZ_PARDOT_ID",
			Value: "operator",
		},
		{
			Name:  "HZ_PHONE_HOME_ENABLED",
			Value: strconv.FormatBool(util.IsPhoneHomeEnabled()),
		},
	}
	if h.Spec.LicenseKeySecret != "" {
		envs = append(envs,
			v1.EnvVar{
				Name: hzLicenseKey,
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: h.Spec.LicenseKeySecret,
						},
						Key: n.LicenseDataKey,
					},
				},
			})
	}

	return envs
}

func labels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	return map[string]string{
		n.ApplicationNameLabel:         n.Hazelcast,
		n.ApplicationInstanceNameLabel: h.Name,
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
}

func statefulSetAnnotations(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	ans := map[string]string{}
	if h.Spec.ExposeExternally.IsSmart() {
		ans[n.ServicePerPodCountAnnotation] = strconv.Itoa(int(h.Spec.ClusterSize))
	}
	return ans
}

func podAnnotations(h *hazelcastv1alpha1.Hazelcast) (map[string]string, error) {
	ans := map[string]string{}
	if h.Spec.ExposeExternally.IsSmart() {
		ans[n.ExposeExternallyAnnotation] = string(h.Spec.ExposeExternally.MemberAccessType())
	}
	cfg := config.HazelcastWrapper{Hazelcast: hazelcastConfigMapStruct(h).HazelcastConfigForcingRestart()}
	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	ans[n.CurrentHazelcastConfigForcingRestartChecksum] = fmt.Sprint(crc32.ChecksumIEEE(cfgYaml))

	return ans, nil
}

func metadata(h *hazelcastv1alpha1.Hazelcast) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      h.Name,
		Namespace: h.Namespace,
		Labels:    labels(h),
	}
}

func (r *HazelcastReconciler) updateLastSuccessfulConfiguration(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	hs, err := json.Marshal(h.Spec)
	if err != nil {
		return err
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, h, func() error {
		if h.ObjectMeta.Annotations == nil {
			ans := map[string]string{}
			h.ObjectMeta.Annotations = ans
		}
		h.ObjectMeta.Annotations[n.LastSuccessfulSpecAnnotation] = string(hs)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Hazelcast Annotation", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) applyDefaultHazelcastSpecs(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {
	changed := false
	if h.Spec.Repository == "" {
		h.Spec.Repository = n.HazelcastRepo
		changed = true
	}
	if h.Spec.Version == "" {
		h.Spec.Version = n.HazelcastVersion
		changed = true
	}
	if h.Spec.ImagePullPolicy == "" {
		h.Spec.ImagePullPolicy = n.HazelcastImagePullPolicy
		changed = true
	}
	if h.Spec.ClusterSize == 0 {
		h.Spec.ClusterSize = n.DefaultClusterSize
		changed = true
	}
	if h.Spec.ClusterName == "" {
		h.Spec.ClusterName = n.DefaultClusterName
	}
	if !changed {
		return nil
	}
	return r.Update(ctx, h)
}
