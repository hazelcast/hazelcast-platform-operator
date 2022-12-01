package hazelcast

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net"
	"path"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	proto "github.com/hazelcast/hazelcast-go-client"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

// Environment variables used for Hazelcast cluster configuration
const (
	// hzLicenseKey License key for Hazelcast cluster
	hzLicenseKey = "HZ_LICENSEKEY"
)

func (r *HazelcastReconciler) executeFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(h, n.Finalizer) {
		return nil
	}
	if err := r.deleteDependentCRs(ctx, h); err != nil {
		return fmt.Errorf("Could not delete all dependent CRs: %w", err)
	}
	if err := r.removeClusterRole(ctx, h, logger); err != nil {
		return fmt.Errorf("ClusterRole could not be removed: %w", err)
	}
	if err := r.removeClusterRoleBinding(ctx, h, logger); err != nil {
		return fmt.Errorf("ClusterRoleBinding could not be removed: %w", err)
	}
	lk := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	r.statusServiceRegistry.Delete(lk)
	r.clientRegistry.Delete(ctx, lk)

	controllerutil.RemoveFinalizer(h, n.Finalizer)
	err := r.Update(ctx, h)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *HazelcastReconciler) deleteDependentCRs(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {

	dependentCRs := map[string]client.ObjectList{
		"Map":           &hazelcastv1alpha1.MapList{},
		"MultiMap":      &hazelcastv1alpha1.MultiMapList{},
		"Topic":         &hazelcastv1alpha1.TopicList{},
		"ReplicatedMap": &hazelcastv1alpha1.ReplicatedMapList{},
		"Queue":         &hazelcastv1alpha1.QueueList{},
		"Cache":         &hazelcastv1alpha1.CacheList{},
	}
	for crKind, crList := range dependentCRs {
		if err := r.deleteDependentCR(ctx, h, crKind, crList); err != nil {
			return err
		}
	}
	return nil
}

func (r *HazelcastReconciler) deleteDependentCR(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, crKind string, objList client.ObjectList) error {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	if err := r.Client.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("could not get Hazelcast dependent %v resources %w", crKind, err)
	}

	dsItems := objList.(CRLister).GetItems()
	if len(dsItems) == 0 {
		return nil
	}

	g, groupCtx := errgroup.WithContext(ctx)
	for i := 0; i < len(dsItems); i++ {
		i := i
		g.Go(func() error {
			return util.DeleteObject(groupCtx, r.Client, dsItems[i])
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error deleting %v resources %w", crKind, err)
	}

	if err := r.Client.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("hazelcast dependent %v resources are not deleted yet %w", crKind, err)
	}

	dsItems = objList.(CRLister).GetItems()
	if len(dsItems) != 0 {
		return fmt.Errorf("hazelcast dependent %v resources are not deleted yet", crKind)
	}

	return nil
}

func (r *HazelcastReconciler) removeClusterRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	clusterRole := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: h.ClusterScopedName()}, clusterRole)
	if err != nil && kerrors.IsNotFound(err) {
		logger.V(util.DebugLevel).Info("ClusterRole is not created yet. Or it is already removed.")
		return nil
	}

	err = r.Delete(ctx, clusterRole)
	if err != nil {
		return fmt.Errorf("failed to clean up ClusterRole: %w", err)
	}
	logger.V(util.DebugLevel).Info("ClusterRole removed successfully")
	return nil
}

func (r *HazelcastReconciler) removeClusterRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	crb := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: h.ClusterScopedName()}, crb)
	if err != nil && kerrors.IsNotFound(err) {
		logger.V(util.DebugLevel).Info("ClusterRoleBinding is not created yet. Or it is already removed.")
		return nil
	}

	err = r.Delete(ctx, crb)
	if err != nil {
		return fmt.Errorf("failed to clean up ClusterRoleBinding: %w", err)
	}
	logger.V(util.DebugLevel).Info("ClusterRoleBinding removed successfully")
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

	if h.Spec.Persistence.IsEnabled() {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"apps"},
			Resources: []string{"statefulsets"},
			Verbs:     []string{"watch", "list"},
		})
	}

	if platform.GetType() == platform.OpenShift {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"security.openshift.io"},
			Resources: []string{"securitycontextconstraints"},
			Verbs:     []string{"use"},
		})
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, clusterRole, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ClusterRole", h.ClusterScopedName(), "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileRole(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labels(h),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
		},
	}

	err := controllerutil.SetControllerReference(h, role, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, role, func() error {
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Role", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileServiceAccount(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metadata(h),
	}

	err := controllerutil.SetControllerReference(h, serviceAccount, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on ServiceAccount: %w", err)
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

func (r *HazelcastReconciler) reconcileRoleBinding(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    labels(h),
		},
	}

	err := controllerutil.SetControllerReference(h, rb, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, rb, func() error {
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      h.Name,
				Namespace: h.Namespace,
			},
		}
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     h.Name,
		}

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "RoleBinding", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileService(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	service := &corev1.Service{
		ObjectMeta: metadata(h),
		Spec: corev1.ServiceSpec{
			Selector: labels(h),
			Ports:    hazelcastPort(),
		},
	}

	if h.ExternalAddressEnabled() && !h.Spec.ExposeExternally.IsSmart() {
		service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
			Name:       "hazelcast-port-ex",
			Protocol:   corev1.ProtocolTCP,
			Port:       5702,
			TargetPort: intstr.FromInt(5702),
		})
	}

	if serviceType(h) == corev1.ServiceTypeClusterIP {
		// We want to use headless to be compatible with Hazelcast helm chart
		service.Spec.ClusterIP = "None"
	}

	err := controllerutil.SetControllerReference(h, service, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Service: %w", err)
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

	for i := 0; i < int(*h.Spec.ClusterSize); i++ {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      servicePerPodName(i, h),
				Namespace: h.Namespace,
				Labels:    servicePerPodLabels(h),
			},
			Spec: corev1.ServiceSpec{
				Selector:                 servicePerPodSelector(i, h),
				Ports:                    hazelcastPort(),
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
		s = int(*h.Spec.ClusterSize)
	}

	// Delete unused services (when the cluster was scaled down)
	// The current number of service per pod is always stored in the StatefulSet annotations
	sts := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: h.Name, Namespace: h.Namespace}, sts)
	if err != nil {
		if kerrors.IsNotFound(err) {
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
			if kerrors.IsNotFound(err) {
				// Not found, no need to remove the service
				continue
			}
			return err
		}
		err = r.Client.Delete(ctx, s)
		if err != nil {
			if kerrors.IsNotFound(err) {
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

func hazelcastPort() []v1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:        n.HazelcastPortName,
			Protocol:    corev1.ProtocolTCP,
			Port:        n.DefaultHzPort,
			TargetPort:  intstr.FromString(n.Hazelcast),
			AppProtocol: pointer.String("tcp"),
		},
	}
}

func (r *HazelcastReconciler) isServicePerPodReady(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) bool {
	if !h.Spec.ExposeExternally.IsSmart() {
		// Service per pod applies only to Smart type
		return true
	}

	// Check if each service per pod is ready
	for i := 0; i < int(*h.Spec.ClusterSize); i++ {
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
			for _, ingress := range s.Status.LoadBalancer.Ingress {
				// Hostname is set for load-balancer ingress points that are DNS based
				// (typically AWS load-balancers)
				if ingress.Hostname != "" {
					if _, err := net.DefaultResolver.LookupHost(ctx, ingress.Hostname); err != nil {
						// Hostname does not resolve yet
						return false
					}
				}
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
		return fmt.Errorf("failed to set owner reference on ConfigMap: %w", err)
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data, err = hazelcastConfigMapData(ctx, r.Client, h)
		return err
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "ConfigMap", h.Name, "result", opResult)
	}
	return err
}

func hazelcastConfigMapData(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) (map[string]string, error) {
	cfg := hazelcastConfigMapStruct(h)

	fillHazelcastConfigWithProperties(&cfg, h)
	fillHazelcastConfigWithExecutorServices(&cfg, h)

	mapList := &hazelcastv1alpha1.MapList{}
	if err := c.List(ctx, mapList, client.MatchingFields{"hazelcastResourceName": h.Name}); err != nil {
		return nil, err
	}

	ml := filterPersistedMaps(mapList.Items)

	if err := fillHazelcastConfigWithMaps(ctx, c, &cfg, h, ml); err != nil {
		return nil, err
	}

	dataStructures := []client.ObjectList{
		&hazelcastv1alpha1.MultiMapList{},
		&hazelcastv1alpha1.TopicList{},
		&hazelcastv1alpha1.ReplicatedMapList{},
		&hazelcastv1alpha1.QueueList{},
		&hazelcastv1alpha1.CacheList{},
	}
	for _, ds := range dataStructures {
		filteredDSList, err := filterPersistedDS(ctx, c, h.Name, ds)
		if err != nil {
			return nil, err
		}
		if len(filteredDSList) == 0 {
			continue
		}
		switch filteredDSList[0].(Type).GetKind() {
		case "MultiMap":
			fillHazelcastConfigWithMultiMaps(&cfg, filteredDSList)
		case "Topic":
			fillHazelcastConfigWithTopics(&cfg, filteredDSList)
		case "ReplicatedMap":
			fillHazelcastConfigWithReplicatedMaps(&cfg, filteredDSList)
		case "Queue":
			fillHazelcastConfigWithQueues(&cfg, filteredDSList)
		case "Cache":
			fillHazelcastConfigWithCaches(&cfg, filteredDSList)
		}
	}

	yml, err := yaml.Marshal(config.HazelcastWrapper{Hazelcast: cfg})
	if err != nil {
		return nil, err
	}
	return map[string]string{"hazelcast.yaml": string(yml)}, nil
}

func hazelcastConfigMapStruct(h *hazelcastv1alpha1.Hazelcast) config.Hazelcast {
	cfg := config.Hazelcast{
		Network: config.Network{
			Join: config.Join{
				Kubernetes: config.Kubernetes{
					Enabled:     pointer.Bool(true),
					ServiceName: h.Name,
				},
			},
			RestAPI: config.RestAPI{
				Enabled: pointer.Bool(true),
				EndpointGroups: config.EndpointGroups{
					HealthCheck: config.EndpointGroup{
						Enabled: pointer.Bool(true),
					},
					ClusterWrite: config.EndpointGroup{
						Enabled: pointer.Bool(true),
					},
					Persistence: config.EndpointGroup{
						Enabled: pointer.Bool(true),
					},
				},
			},
		},
	}

	if h.Spec.JetEngineConfiguration.IsConfigured() {
		cfg.Jet = config.Jet{
			Enabled:               h.Spec.JetEngineConfiguration.Enabled,
			ResourceUploadEnabled: pointer.Bool(h.Spec.JetEngineConfiguration.ResourceUploadEnabled),
		}
	}

	if h.Spec.UserCodeDeployment != nil {
		cfg.UserCodeDeployment = config.UserCodeDeployment{
			Enabled: h.Spec.UserCodeDeployment.ClientEnabled,
		}
	}

	if h.Spec.ExposeExternally.UsesNodeName() {
		cfg.Network.Join.Kubernetes.UseNodeNameAsExternalAddress = pointer.Bool(true)
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
			Enabled:                   pointer.Bool(true),
			BaseDir:                   h.Spec.Persistence.BaseDir,
			BackupDir:                 h.Spec.Persistence.BaseDir + "/hot-backup",
			Parallelism:               1,
			ValidationTimeoutSec:      120,
			DataLoadTimeoutSec:        900,
			ClusterDataRecoveryPolicy: clusterDataRecoveryPolicy(h.Spec.Persistence.ClusterDataRecoveryPolicy),
			AutoRemoveStaleData:       &[]bool{h.Spec.Persistence.AutoRemoveStaleData()}[0],
		}
		if h.Spec.Persistence.DataRecoveryTimeout != 0 {
			cfg.Persistence.ValidationTimeoutSec = h.Spec.Persistence.DataRecoveryTimeout
			cfg.Persistence.DataLoadTimeoutSec = h.Spec.Persistence.DataRecoveryTimeout
		}
	}
	return cfg
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

func filterProperties(p map[string]string) map[string]string {
	filteredProperties := map[string]string{}
	for propertyKey, value := range p {
		if _, ok := hazelcastv1alpha1.BlackListProperties[propertyKey]; !ok {
			filteredProperties[propertyKey] = value
		}
	}
	return filteredProperties
}

func filterPersistedMaps(ml []hazelcastv1alpha1.Map) []hazelcastv1alpha1.Map {
	l := make([]hazelcastv1alpha1.Map, 0)

	for _, mp := range ml {
		switch mp.Status.State {
		case hazelcastv1alpha1.MapPersisting, hazelcastv1alpha1.MapSuccess:
			l = append(l, mp)
		case hazelcastv1alpha1.MapFailed, hazelcastv1alpha1.MapPending:
			if spec, ok := mp.Annotations[n.LastSuccessfulSpecAnnotation]; ok {
				ms := &hazelcastv1alpha1.MapSpec{}
				err := json.Unmarshal([]byte(spec), ms)
				if err != nil {
					continue
				}
				mp.Spec = *ms
				l = append(l, mp)
			}
		default:
		}
	}
	return l
}

func filterPersistedDS(ctx context.Context, c client.Client, hzResourceName string, objList client.ObjectList) ([]client.Object, error) {
	if err := c.List(ctx, objList, client.MatchingFields{"hazelcastResourceName": hzResourceName}); err != nil {
		return nil, err
	}
	l := make([]client.Object, 0)
	for _, obj := range objList.(CRLister).GetItems() {
		if isDSPersisted(obj) {
			l = append(l, obj)
		}
	}
	return l, nil
}

func fillHazelcastConfigWithProperties(cfg *config.Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	p := filterProperties(h.Spec.Properties)
	p["hazelcast.persistence.auto.cluster.state"] = "false"
	cfg.Properties = p
}

func fillHazelcastConfigWithMaps(ctx context.Context, c client.Client, cfg *config.Hazelcast, h *hazelcastv1alpha1.Hazelcast, ml []hazelcastv1alpha1.Map) error {
	if len(ml) != 0 {
		cfg.Map = map[string]config.Map{}
		for _, mcfg := range ml {
			m, err := createMapConfig(ctx, c, h, &mcfg)
			if err != nil {
				return err
			}
			cfg.Map[mcfg.MapName()] = m
		}
	}
	return nil
}

func fillHazelcastConfigWithMultiMaps(cfg *config.Hazelcast, mml []client.Object) {
	if len(mml) != 0 {
		cfg.MultiMap = map[string]config.MultiMap{}
		for _, mm := range mml {
			mm := mm.(*hazelcastv1alpha1.MultiMap)
			mmcfg := createMultiMapConfig(mm)
			cfg.MultiMap[mm.GetDSName()] = mmcfg
		}
	}
}

func fillHazelcastConfigWithTopics(cfg *config.Hazelcast, tl []client.Object) {
	if len(tl) != 0 {
		cfg.Topic = map[string]config.Topic{}
		for _, t := range tl {
			t := t.(*hazelcastv1alpha1.Topic)
			tcfg := createTopicConfig(t)
			cfg.Topic[t.GetDSName()] = tcfg
		}
	}
}

func fillHazelcastConfigWithQueues(cfg *config.Hazelcast, ql []client.Object) {
	if len(ql) != 0 {
		cfg.Queue = map[string]config.Queue{}
		for _, q := range ql {
			q := q.(*hazelcastv1alpha1.Queue)
			qcfg := createQueueConfig(q)
			cfg.Queue[q.GetDSName()] = qcfg
		}
	}
}

func fillHazelcastConfigWithCaches(cfg *config.Hazelcast, cl []client.Object) {
	if len(cl) != 0 {
		cfg.Cache = map[string]config.Cache{}
		for _, c := range cl {
			c := c.(*hazelcastv1alpha1.Cache)
			ccfg := createCacheConfig(c)
			cfg.Cache[c.GetDSName()] = ccfg
		}
	}
}

func fillHazelcastConfigWithReplicatedMaps(cfg *config.Hazelcast, rml []client.Object) {
	if len(rml) != 0 {
		cfg.ReplicatedMap = map[string]config.ReplicatedMap{}
		for _, rm := range rml {
			rm := rm.(*hazelcastv1alpha1.ReplicatedMap)
			rmcfg := createReplicatedMapConfig(rm)
			cfg.ReplicatedMap[rm.GetDSName()] = rmcfg
		}
	}
}

func fillHazelcastConfigWithExecutorServices(cfg *config.Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	if len(h.Spec.ExecutorServices) != 0 {
		cfg.ExecutorService = map[string]config.ExecutorService{}
		for _, escfg := range h.Spec.ExecutorServices {
			cfg.ExecutorService[escfg.Name] = createExecutorServiceConfig(&escfg)
		}
	}

	if len(h.Spec.DurableExecutorServices) != 0 {
		cfg.DurableExecutorService = map[string]config.DurableExecutorService{}
		for _, descfg := range h.Spec.DurableExecutorServices {
			cfg.DurableExecutorService[descfg.Name] = createDurableExecutorServiceConfig(&descfg)
		}
	}

	if len(h.Spec.ScheduledExecutorServices) != 0 {
		cfg.ScheduledExecutorService = map[string]config.ScheduledExecutorService{}
		for _, sescfg := range h.Spec.ScheduledExecutorServices {
			cfg.ScheduledExecutorService[sescfg.Name] = createScheduledExecutorServiceConfig(&sescfg)
		}
	}
}

func createMapConfig(ctx context.Context, c client.Client, hz *hazelcastv1alpha1.Hazelcast, m *hazelcastv1alpha1.Map) (config.Map, error) {
	ms := m.Spec
	mc := config.Map{
		BackupCount:       *ms.BackupCount,
		AsyncBackupCount:  *ms.AsyncBackupCount,
		TimeToLiveSeconds: *ms.TimeToLiveSeconds,
		ReadBackupData:    false,
		Eviction: config.MapEviction{
			Size:           *ms.Eviction.MaxSize,
			MaxSizePolicy:  string(ms.Eviction.MaxSizePolicy),
			EvictionPolicy: string(ms.Eviction.EvictionPolicy),
		},
		InMemoryFormat:    string(ms.InMemoryFormat),
		Indexes:           copyMapIndexes(ms.Indexes),
		StatisticsEnabled: true,
		DataPersistence: config.DataPersistence{
			Enabled: ms.PersistenceEnabled,
			Fsync:   false,
		},
	}

	if util.IsEnterprise(hz.Spec.Repository) {
		mc.WanReplicationReference = wanReplicationRef(defaultWanReplicationRefCodec(hz, m))
	}

	if ms.MapStore != nil {
		msp, err := getMapStoreProperties(ctx, c, ms.MapStore.PropertiesSecretName, hz.Namespace)
		if err != nil {
			return config.Map{}, err
		}
		mc.MapStoreConfig = config.MapStoreConfig{
			Enabled:           true,
			WriteCoalescing:   ms.MapStore.WriteCoealescing,
			WriteDelaySeconds: ms.MapStore.WriteDelaySeconds,
			WriteBatchSize:    ms.MapStore.WriteBatchSize,
			ClassName:         ms.MapStore.ClassName,
			Properties:        msp,
			InitialLoadMode:   string(ms.MapStore.InitialMode),
		}

	}

	if len(ms.EntryListeners) != 0 {
		mc.EntryListeners = make([]config.EntryListener, 0, len(ms.EntryListeners))
		for _, el := range ms.EntryListeners {
			mc.EntryListeners = append(mc.EntryListeners, config.EntryListener{
				ClassName:    el.ClassName,
				IncludeValue: el.GetIncludedValue(),
				Local:        el.Local,
			})
		}
	}
	return mc, nil
}

func wanReplicationRef(ref codecTypes.WanReplicationRef) map[string]config.WanReplicationReference {
	return map[string]config.WanReplicationReference{
		ref.Name: {
			MergePolicyClassName: ref.MergePolicyClassName,
			RepublishingEnabled:  ref.RepublishingEnabled,
			Filters:              ref.Filters,
		},
	}
}

func getMapStoreProperties(ctx context.Context, c client.Client, sn, ns string) (map[string]string, error) {
	if sn == "" {
		return nil, nil
	}
	s := &v1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: sn, Namespace: ns}, s)
	if err != nil {
		return nil, err
	}

	props := map[string]string{}
	for k, v := range s.Data {
		props[k] = string(v)
	}
	return props, nil
}

func copyMapIndexes(idx []hazelcastv1alpha1.IndexConfig) []config.MapIndex {
	if idx == nil {
		return nil
	}
	ics := make([]config.MapIndex, len(idx))
	for i, index := range idx {
		ics[i].Type = string(index.Type)
		ics[i].Attributes = index.Attributes
		ics[i].Name = index.Name
		if index.BitmapIndexOptions != nil {
			ics[i].BitmapIndexOptions.UniqueKey = index.BitmapIndexOptions.UniqueKey
			ics[i].BitmapIndexOptions.UniqueKeyTransformation = string(index.BitmapIndexOptions.UniqueKeyTransition)
		}
	}

	return ics
}

func createExecutorServiceConfig(es *hazelcastv1alpha1.ExecutorServiceConfiguration) config.ExecutorService {
	return config.ExecutorService{PoolSize: es.PoolSize, QueueCapacity: es.QueueCapacity}
}

func createDurableExecutorServiceConfig(des *hazelcastv1alpha1.DurableExecutorServiceConfiguration) config.DurableExecutorService {
	return config.DurableExecutorService{PoolSize: des.PoolSize, Durability: des.Durability, Capacity: des.Capacity}
}

func createScheduledExecutorServiceConfig(ses *hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) config.ScheduledExecutorService {
	return config.ScheduledExecutorService{PoolSize: ses.PoolSize, Durability: ses.Durability, Capacity: ses.Capacity, CapacityPolicy: ses.CapacityPolicy}
}

func createMultiMapConfig(mm *hazelcastv1alpha1.MultiMap) config.MultiMap {
	mms := mm.Spec
	return config.MultiMap{
		BackupCount:       *mms.BackupCount,
		AsyncBackupCount:  *mms.AsyncBackupCount,
		Binary:            mms.Binary,
		CollectionType:    string(mms.CollectionType),
		StatisticsEnabled: n.DefaultMultiMapStatisticsEnabled,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultMultiMapMergePolicy,
			BatchSize: n.DefaultMultiMapMergeBatchSize,
		},
	}
}

func createQueueConfig(q *hazelcastv1alpha1.Queue) config.Queue {
	qs := q.Spec
	return config.Queue{
		BackupCount:             *qs.BackupCount,
		AsyncBackupCount:        *qs.AsyncBackupCount,
		EmptyQueueTtl:           *qs.EmptyQueueTtlSeconds,
		MaxSize:                 *qs.MaxSize,
		StatisticsEnabled:       n.DefaultQueueStatisticsEnabled,
		PriorityComparatorClass: qs.PriorityComparatorClassName,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultQueueMergePolicy,
			BatchSize: n.DefaultQueueMergeBatchSize,
		},
	}
}

func createCacheConfig(c *hazelcastv1alpha1.Cache) config.Cache {
	cs := c.Spec
	cache := config.Cache{
		BackupCount:       *cs.BackupCount,
		AsyncBackupCount:  *cs.AsyncBackupCount,
		StatisticsEnabled: n.DefaultCacheStatisticsEnabled,
		ManagementEnabled: n.DefaultCacheManagementEnabled,
		ReadThrough:       n.DefaultCacheReadThrough,
		WriteThrough:      n.DefaultCacheWriteThrough,
		InMemoryFormat:    n.DefaultCacheInMemoryFormat,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultCacheMergePolicy,
			BatchSize: n.DefaultCacheMergeBatchSize,
		},
		DataPersistence: config.DataPersistence{
			Enabled: cs.PersistenceEnabled,
			Fsync:   false,
		},
	}
	if cs.KeyType != "" {
		cache.KeyType = config.ClassType{
			ClassName: cs.KeyType,
		}
	}
	if cs.ValueType != "" {
		cache.ValueType = config.ClassType{
			ClassName: cs.ValueType,
		}
	}

	return cache
}

func createTopicConfig(t *hazelcastv1alpha1.Topic) config.Topic {
	ts := t.Spec
	return config.Topic{
		GlobalOrderingEnabled: ts.GlobalOrderingEnabled,
		MultiThreadingEnabled: ts.MultiThreadingEnabled,
		StatisticsEnabled:     n.DefaultTopicStatisticsEnabled,
	}
}

func createReplicatedMapConfig(rm *hazelcastv1alpha1.ReplicatedMap) config.ReplicatedMap {
	rms := rm.Spec
	return config.ReplicatedMap{
		InMemoryFormat:    string(rms.InMemoryFormat),
		AsyncFillup:       rms.AsyncFillup,
		StatisticsEnabled: n.DefaultReplicatedMapStatisticsEnabled,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultReplicatedMapMergePolicy,
			BatchSize: n.DefaultReplicatedMapMergeBatchSize,
		},
	}
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
					ServiceAccountName: h.Name,
					SecurityContext: &v1.PodSecurityContext{
						FSGroup:      pointer.Int64(65534),
						RunAsNonRoot: pointer.Bool(true),
						RunAsUser:    pointer.Int64(65534),
					},
					Containers: []v1.Container{{
						Name: n.Hazelcast,
						Ports: []v1.ContainerPort{{
							ContainerPort: n.DefaultHzPort,
							Name:          n.Hazelcast,
							Protocol:      v1.ProtocolTCP,
						}},
						LivenessProbe: &v1.Probe{
							ProbeHandler: v1.ProbeHandler{
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
							ProbeHandler: v1.ProbeHandler{
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
						SecurityContext: containerSecurityContext(h),
					}},
					TerminationGracePeriodSeconds: pointer.Int64(600),
				},
			},
		},
	}

	if h.Spec.Persistence.IsEnabled() {
		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, backupAgentContainer(h))
		if !h.Spec.Persistence.UseHostPath() {
			sts.Spec.VolumeClaimTemplates = persistentVolumeClaim(h)
		}
	}

	err := controllerutil.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Statefulset: %w", err)
	}

	opResult, err := util.CreateOrUpdate(ctx, r.Client, sts, func() error {
		sts.Spec.Replicas = h.Spec.ClusterSize
		sts.ObjectMeta.Annotations = statefulSetAnnotations(h)
		sts.Spec.Template.Annotations, err = podAnnotations(sts.Spec.Template.Annotations, h)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.ImagePullSecrets = h.Spec.ImagePullSecrets
		sts.Spec.Template.Spec.Containers[0].Image = h.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env = env(h)
		sts.Spec.Template.Spec.Containers[0].ImagePullPolicy = h.Spec.ImagePullPolicy

		if h.Spec.Scheduling != nil {
			sts.Spec.Template.Spec.Affinity = h.Spec.Scheduling.Affinity
			sts.Spec.Template.Spec.Tolerations = h.Spec.Scheduling.Tolerations
			sts.Spec.Template.Spec.NodeSelector = h.Spec.Scheduling.NodeSelector
			sts.Spec.Template.Spec.TopologySpreadConstraints = h.Spec.Scheduling.TopologySpreadConstraints
		} else {
			sts.Spec.Template.Spec.Affinity = nil
			sts.Spec.Template.Spec.Tolerations = nil
			sts.Spec.Template.Spec.NodeSelector = nil
			sts.Spec.Template.Spec.TopologySpreadConstraints = nil
		}

		if h.Spec.Resources != nil {
			sts.Spec.Template.Spec.Containers[0].Resources = *h.Spec.Resources
		} else {
			sts.Spec.Template.Spec.Containers[0].Resources = v1.ResourceRequirements{}
		}

		sts.Spec.Template.Spec.InitContainers, err = initContainers(ctx, h, r.Client)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.Volumes = volumes(h)
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts(h)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", h.Name, "result", opResult)
	}
	return err
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
						corev1.ResourceStorage: *h.Spec.Persistence.Pvc.RequestStorage,
					},
				},
				StorageClassName: h.Spec.Persistence.Pvc.StorageClassName,
			},
		},
	}
}

func backupAgentContainer(h *hazelcastv1alpha1.Hazelcast) v1.Container {
	return v1.Container{
		Name:  n.BackupAgent,
		Image: h.AgentDockerImage(),
		Ports: []v1.ContainerPort{{
			ContainerPort: n.DefaultAgentPort,
			Name:          n.BackupAgent,
			Protocol:      v1.ProtocolTCP,
		}},
		Args: []string{"backup"},
		LivenessProbe: &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(8080),
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
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 0,
			TimeoutSeconds:      10,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    10,
		},
		Env: []v1.EnvVar{
			{
				Name:  "BACKUP_CA",
				Value: path.Join(n.MTLSCertPath, "ca.crt"),
			},
			{
				Name:  "BACKUP_CERT",
				Value: path.Join(n.MTLSCertPath, "tls.crt"),
			},
			{
				Name:  "BACKUP_KEY",
				Value: path.Join(n.MTLSCertPath, "tls.key"),
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      n.PersistenceVolumeName,
				MountPath: h.Spec.Persistence.BaseDir,
			},
			{
				Name:      n.MTLSCertSecretName,
				MountPath: n.MTLSCertPath,
			},
		},
		SecurityContext: containerSecurityContext(h),
	}
}

func containerSecurityContext(h *hazelcastv1alpha1.Hazelcast) *v1.SecurityContext {
	sec := &v1.SecurityContext{
		RunAsNonRoot:             pointer.Bool(true),
		RunAsUser:                pointer.Int64(65534),
		Privileged:               pointer.Bool(false),
		ReadOnlyRootFilesystem:   pointer.Bool(true),
		AllowPrivilegeEscalation: pointer.Bool(false),
		Capabilities: &v1.Capabilities{
			Drop: []v1.Capability{"ALL"},
		},
	}

	if !h.Spec.Persistence.IsEnabled() {
		return sec
	}

	sec.ReadOnlyRootFilesystem = pointer.Bool(false)

	if !h.Spec.Persistence.UseHostPath() {
		return sec
	}

	sec.RunAsNonRoot = pointer.Bool(false)
	sec.RunAsUser = pointer.Int64(0)

	if platform.GetType() == platform.OpenShift {
		sec.Privileged = pointer.Bool(true)
		sec.AllowPrivilegeEscalation = pointer.Bool(true)
	}

	return sec
}

func initContainers(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, cl client.Client) ([]corev1.Container, error) {
	var containers []corev1.Container

	if h.Spec.UserCodeDeployment.IsBucketEnabled() {
		containers = append(containers, ucdAgentContainer(h))
	}

	if !h.Spec.Persistence.IsRestoreEnabled() {
		return containers, nil
	}

	if h.Spec.Persistence.RestoreFromHotBackupResourceName() {
		cont, err := getRestoreContainerFromHotBackupResource(ctx, cl, h,
			types.NamespacedName{Namespace: h.Namespace, Name: h.Spec.Persistence.Restore.HotBackupResourceName})
		if err != nil {
			return nil, err
		}
		containers = append(containers, cont)

		return containers, nil
	}

	// restoring from bucket config
	containers = append(containers, restoreAgentContainer(h, h.Spec.Persistence.Restore.BucketConfiguration.Secret,
		h.Spec.Persistence.Restore.BucketConfiguration.BucketURI))

	return containers, nil
}

func getRestoreContainerFromHotBackupResource(ctx context.Context, cl client.Client, h *hazelcastv1alpha1.Hazelcast, key types.NamespacedName) (corev1.Container, error) {
	hb := &hazelcastv1alpha1.HotBackup{}
	err := cl.Get(ctx, key, hb)
	if err != nil {
		return corev1.Container{}, err
	}

	var cont corev1.Container
	if hb.Spec.IsExternal() {
		bucketURI := hb.Status.GetBucketURI()
		cont = restoreAgentContainer(h, hb.Spec.Secret, bucketURI)
	} else {
		backupFolder := hb.Status.GetBackupFolder()
		cont = restoreLocalAgentContainer(h, backupFolder)
	}

	return cont, nil
}

func restoreAgentContainer(h *hazelcastv1alpha1.Hazelcast, secretName, bucket string) v1.Container {
	commandName := "restore_pvc"

	if h.Spec.Persistence.UseHostPath() {
		commandName = "restore_hostpath"
	}

	return v1.Container{
		Name:            n.RestoreAgent,
		Image:           h.AgentDockerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            []string{commandName},
		Env: []v1.EnvVar{
			{
				Name:  "RESTORE_SECRET_NAME",
				Value: secretName,
			},
			{
				Name:  "RESTORE_BUCKET",
				Value: bucket,
			},
			{
				Name:  "RESTORE_DESTINATION",
				Value: h.Spec.Persistence.BaseDir,
			},
			{
				Name:  "RESTORE_ID",
				Value: string(h.Spec.Persistence.Restore.Hash()),
			},
			{
				Name: "RESTORE_HOSTNAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []v1.VolumeMount{{
			Name:      n.PersistenceVolumeName,
			MountPath: h.Spec.Persistence.BaseDir,
		}},
		SecurityContext: containerSecurityContext(h),
	}
}

func restoreLocalAgentContainer(h *hazelcastv1alpha1.Hazelcast, backupFolder string) v1.Container {
	commandName := "restore_pvc_local"

	if h.Spec.Persistence.UseHostPath() {
		commandName = "restore_hostpath_local"
	}

	return v1.Container{
		Name:            n.RestoreLocalAgent,
		Image:           h.AgentDockerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            []string{commandName},
		Env: []v1.EnvVar{
			{
				Name:  "RESTORE_LOCAL_BACKUP_FOLDER_NAME",
				Value: backupFolder,
			},
			{
				Name:  "RESTORE_LOCAL_BACKUP_BASE_DIR",
				Value: h.Spec.Persistence.BaseDir,
			},
			{
				Name:  "RESTORE_LOCAL_ID",
				Value: string(h.Spec.Persistence.Restore.Hash()),
			},
			{
				Name: "RESTORE_LOCAL_HOSTNAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []v1.VolumeMount{{
			Name:      n.PersistenceVolumeName,
			MountPath: h.Spec.Persistence.BaseDir,
		}},
		SecurityContext: containerSecurityContext(h),
	}
}

func ucdAgentContainer(h *hazelcastv1alpha1.Hazelcast) v1.Container {
	return v1.Container{
		Name:  n.UserCodeDownloadAgent + h.Spec.UserCodeDeployment.TriggerSequence,
		Image: h.AgentDockerImage(),
		Args:  []string{"user-code-deployment"},
		Env: []v1.EnvVar{
			{
				Name:  "UCD_SECRET_NAME",
				Value: h.Spec.UserCodeDeployment.BucketConfiguration.Secret,
			},
			{
				Name:  "UCD_BUCKET",
				Value: h.Spec.UserCodeDeployment.BucketConfiguration.BucketURI,
			},
			{
				Name:  "UCD_DESTINATION",
				Value: n.UserCodeBucketPath,
			},
		},
		VolumeMounts: []v1.VolumeMount{ucdAgentVolumeMount(h)},
	}
}

func ucdAgentVolumeMount(_ *hazelcastv1alpha1.Hazelcast) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      n.UserCodeBucketVolumeName,
		MountPath: n.UserCodeBucketPath,
	}
}

func volumes(h *hazelcastv1alpha1.Hazelcast) []v1.Volume {
	vols := []v1.Volume{
		{
			Name: n.HazelcastStorageName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: h.Name,
					},
					DefaultMode: &[]int32{420}[0],
				},
			},
		},
		userCodeAgentVolume(h),
		tlsVolume(h),
	}
	if h.Spec.Persistence.IsEnabled() && h.Spec.Persistence.UseHostPath() {
		vols = append(vols, hostPathVolume(h))
	}
	if h.Spec.UserCodeDeployment.IsConfigMapEnabled() {
		vols = append(vols, userCodeConfigMapVolumes(h)...)
	}
	return vols
}

func userCodeAgentVolume(_ *hazelcastv1alpha1.Hazelcast) v1.Volume {
	return v1.Volume{
		Name: n.UserCodeBucketVolumeName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
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

func tlsVolume(_ *hazelcastv1alpha1.Hazelcast) v1.Volume {
	return v1.Volume{
		Name: n.MTLSCertSecretName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName:  n.MTLSCertSecretName,
				DefaultMode: &[]int32{420}[0],
			},
		},
	}
}

func userCodeConfigMapVolumes(h *hazelcastv1alpha1.Hazelcast) []corev1.Volume {
	var vols []corev1.Volume
	for _, cm := range h.Spec.UserCodeDeployment.ConfigMaps {
		vols = append(vols, corev1.Volume{
			Name: n.UserCodeConfigMapNamePrefix + cm + h.Spec.UserCodeDeployment.TriggerSequence,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: cm,
					},
					DefaultMode: &[]int32{420}[0],
				},
			},
		})
	}
	return vols
}

func volumeMounts(h *hazelcastv1alpha1.Hazelcast) []corev1.VolumeMount {
	mounts := []v1.VolumeMount{
		{
			Name:      n.HazelcastStorageName,
			MountPath: n.HazelcastMountPath,
		},
		ucdAgentVolumeMount(h),
	}
	if h.Spec.Persistence.IsEnabled() {
		mounts = append(mounts, v1.VolumeMount{
			Name:      n.PersistenceVolumeName,
			MountPath: h.Spec.Persistence.BaseDir,
		})
	}

	if h.Spec.UserCodeDeployment.IsConfigMapEnabled() {
		mounts = append(mounts, userCodeConfigMapVolumeMounts(h)...)
	}
	return mounts
}

func userCodeConfigMapVolumeMounts(h *hazelcastv1alpha1.Hazelcast) []corev1.VolumeMount {
	var vms []corev1.VolumeMount
	for _, cm := range h.Spec.UserCodeDeployment.ConfigMaps {
		vms = append(vms, corev1.VolumeMount{
			Name:      n.UserCodeConfigMapNamePrefix + cm + h.Spec.UserCodeDeployment.TriggerSequence,
			MountPath: n.UserCodeConfigMapPath + "/" + cm,
		})
	}
	return vms
}

// checkHotRestart checks if the persistence feature and AutoForceStart is enabled, and pods are failing
// to perform the Force Start action.
func (r *HazelcastReconciler) checkHotRestart(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !h.Spec.Persistence.IsEnabled() || !h.Spec.Persistence.AutoForceStart {
		return nil
	}
	logger.Info("Persistence and AutoForceStart are enabled. Checking for the cluster DataPersistence.")
	for _, member := range h.Status.Members {
		if !member.Ready && member.Reason == "CrashLoopBackOff" {
			logger.Info("Member is crashing with CrashLoopBackOff.",
				"RestartCounts", member.RestartCount, "Message", member.Message)
			err := NewRestClient(h).ForceStart(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *HazelcastReconciler) ensureClusterActive(ctx context.Context, client hzclient.Client, h *hazelcastv1alpha1.Hazelcast) error {
	// make sure restore is active
	if !h.Spec.Persistence.IsRestoreEnabled() {
		return nil
	}

	// make sure restore was successfull
	if h.Status.Restore == nil {
		return nil
	}

	if h.Status.Restore.State != hazelcastv1alpha1.RestoreSucceeded {
		return nil
	}

	if h.Status.Phase == hazelcastv1alpha1.Pending {
		return nil
	}

	// check if all cluster members are in passive state
	for _, member := range h.Status.Members {
		if member.State != hazelcastv1alpha1.NodeStatePassive {
			return nil
		}
	}

	svc := hzclient.NewClusterStateService(client)
	state, err := svc.ClusterState(ctx)
	if err != nil {
		return err
	}
	if state == codecTypes.ClusterStateActive {
		return nil
	}
	return svc.ChangeClusterState(ctx, codecTypes.ClusterStateActive)
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
		{
			Name:  "LOGGING_PATTERN",
			Value: `{"time":"%date{ISO8601}", "logger": "%logger{36}", "level": "%level", "msg": "%enc{%m %xEx}{JSON}"}%n`,
		},
		{
			Name:  "LOGGING_LEVEL",
			Value: string(h.Spec.LoggingLevel),
		},
		{
			Name:  "CLASSPATH",
			Value: javaClassPath(h),
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

func javaClassPath(h *hazelcastv1alpha1.Hazelcast) string {
	b := []string{n.UserCodeBucketPath + "/*"}

	if !h.Spec.UserCodeDeployment.IsConfigMapEnabled() {
		return b[0]
	}

	for _, cm := range h.Spec.UserCodeDeployment.ConfigMaps {
		b = append(b, n.UserCodeConfigMapPath+"/"+cm+"/*")
	}

	return strings.Join(b, ":")
}

func labels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	return map[string]string{
		n.ApplicationNameLabel:         n.Hazelcast,
		n.ApplicationInstanceNameLabel: h.Name,
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
}

func statefulSetAnnotations(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	if !h.Spec.ExposeExternally.IsSmart() {
		return nil
	}

	return map[string]string{
		n.ServicePerPodCountAnnotation: strconv.Itoa(int(*h.Spec.ClusterSize)),
	}
}

func podAnnotations(annotations map[string]string, h *hazelcastv1alpha1.Hazelcast) (map[string]string, error) {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if h.Spec.ExposeExternally.IsSmart() {
		annotations[n.ExposeExternallyAnnotation] = string(h.Spec.ExposeExternally.MemberAccessType())
	}
	cfg := config.HazelcastWrapper{Hazelcast: hazelcastConfigMapStruct(h).HazelcastConfigForcingRestart()}
	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	annotations[n.CurrentHazelcastConfigForcingRestartChecksum] = fmt.Sprint(crc32.ChecksumIEEE(cfgYaml))

	return annotations, nil
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

func (r *HazelcastReconciler) detectNewExecutorServices(h *hazelcastv1alpha1.Hazelcast, rawLastSpec string) (map[string]interface{}, error) {
	hs, err := json.Marshal(h.Spec)

	if err != nil {
		err = fmt.Errorf("error marshaling Hazelcast as JSON: %w", err)
		return nil, err
	}
	if rawLastSpec == string(hs) {
		return nil, nil
	}
	lastSpec := &hazelcastv1alpha1.HazelcastSpec{}
	err = json.Unmarshal([]byte(rawLastSpec), lastSpec)
	if err != nil {
		err = fmt.Errorf("error unmarshaling Last HZ Spec: %w", err)
		return nil, err
	}

	currentSpec := h.Spec

	existExecutorServices := make(map[string]struct{}, len(lastSpec.ExecutorServices))
	newExecutorServices := make([]hazelcastv1alpha1.ExecutorServiceConfiguration, 0, len(currentSpec.ExecutorServices))
	for _, es := range lastSpec.ExecutorServices {
		existExecutorServices[es.Name] = struct{}{}
	}
	for _, es := range currentSpec.ExecutorServices {
		_, ok := existExecutorServices[es.Name]
		if !ok {
			newExecutorServices = append(newExecutorServices, es)
		}
	}

	existExecutorServices = make(map[string]struct{}, len(lastSpec.DurableExecutorServices))
	newDurableExecutorServices := make([]hazelcastv1alpha1.DurableExecutorServiceConfiguration, 0, len(currentSpec.DurableExecutorServices))
	for _, es := range lastSpec.DurableExecutorServices {
		existExecutorServices[es.Name] = struct{}{}
	}
	for _, es := range currentSpec.DurableExecutorServices {
		_, ok := existExecutorServices[es.Name]
		if !ok {
			newDurableExecutorServices = append(newDurableExecutorServices, es)
		}
	}

	existExecutorServices = make(map[string]struct{}, len(lastSpec.ScheduledExecutorServices))
	newScheduledExecutorServices := make([]hazelcastv1alpha1.ScheduledExecutorServiceConfiguration, 0, len(currentSpec.ScheduledExecutorServices))
	for _, es := range lastSpec.ScheduledExecutorServices {
		existExecutorServices[es.Name] = struct{}{}
	}
	for _, es := range currentSpec.ScheduledExecutorServices {
		_, ok := existExecutorServices[es.Name]
		if !ok {
			newScheduledExecutorServices = append(newScheduledExecutorServices, es)
		}
	}

	return map[string]interface{}{"es": newExecutorServices, "des": newDurableExecutorServices, "ses": newScheduledExecutorServices}, nil
}

func (r *HazelcastReconciler) addExecutorServices(ctx context.Context, client hzclient.Client, newExecutorServices map[string]interface{}) {
	var req *proto.ClientMessage
	for _, es := range newExecutorServices["es"].([]hazelcastv1alpha1.ExecutorServiceConfiguration) {
		esInput := codecTypes.DefaultAddExecutorServiceInput()
		fillAddExecutorServiceInput(esInput, es)
		req = codec.EncodeDynamicConfigAddExecutorConfigRequest(esInput)

		for _, member := range client.OrderedMembers() {
			_, err := client.InvokeOnMember(ctx, req, member.UUID, nil)
			if err != nil {
				continue
			}
		}
	}
	for _, des := range newExecutorServices["des"].([]hazelcastv1alpha1.DurableExecutorServiceConfiguration) {
		esInput := codecTypes.DefaultAddDurableExecutorServiceInput()
		fillAddDurableExecutorServiceInput(esInput, des)
		req = codec.EncodeDynamicConfigAddDurableExecutorConfigRequest(esInput)

		for _, member := range client.OrderedMembers() {
			_, err := client.InvokeOnMember(ctx, req, member.UUID, nil)
			if err != nil {
				continue
			}
		}
	}
	for _, ses := range newExecutorServices["ses"].([]hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) {
		esInput := codecTypes.DefaultAddScheduledExecutorServiceInput()
		fillAddScheduledExecutorServiceInput(esInput, ses)
		req = codec.EncodeDynamicConfigAddScheduledExecutorConfigRequest(esInput)

		for _, member := range client.OrderedMembers() {
			_, err := client.InvokeOnMember(ctx, req, member.UUID, nil)
			if err != nil {
				continue
			}
		}
	}
}

func fillAddExecutorServiceInput(esInput *codecTypes.ExecutorServiceConfig, es hazelcastv1alpha1.ExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.QueueCapacity = es.QueueCapacity
}

func fillAddDurableExecutorServiceInput(esInput *codecTypes.DurableExecutorServiceConfig, es hazelcastv1alpha1.DurableExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.Capacity = es.Capacity
	esInput.Durability = es.Durability
}

func fillAddScheduledExecutorServiceInput(esInput *codecTypes.ScheduledExecutorServiceConfig, es hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.Capacity = es.Capacity
	esInput.CapacityPolicy = es.CapacityPolicy
	esInput.Durability = es.Durability
}
