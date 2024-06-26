package hazelcast

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	errs "errors"
	"fmt"
	"hash/crc32"
	"net"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	proto "github.com/hazelcast/hazelcast-go-client"
	clientTypes "github.com/hazelcast/hazelcast-go-client/types"
	"github.com/hazelcast/platform-operator-agent/init/compound"
	downloadurl "github.com/hazelcast/platform-operator-agent/init/file_download_url"
	downloadbucket "github.com/hazelcast/platform-operator-agent/init/jar_download_bucket"
	"github.com/hazelcast/platform-operator-agent/init/restore"
	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
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
	// JavaOpts java options for Hazelcast
	JavaOpts = "JAVA_OPTS"
)

// DefaultProperties are not overridable by the user
var DefaultProperties = map[string]string{
	"hazelcast.cluster.version.auto.upgrade.enabled": "true",
	// https://docs.hazelcast.com/hazelcast/latest/kubernetes/kubernetes-auto-discovery#configuration
	// We added the following properties to here with their default values, because DefaultProperties cannot be overridden
	"hazelcast.persistence.auto.cluster.state": "true",
}

// DefaultJavaOptions are overridable by the user
var DefaultJavaOptions = map[string]string{
	"-Dhazelcast.stale.join.prevention.duration.seconds": "5",
}

func (r *HazelcastReconciler) executeFinalizer(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(h, n.Finalizer) {
		return nil
	}
	if err := r.deleteDependentCRs(ctx, h); err != nil {
		return fmt.Errorf("Could not delete all dependent CRs: %w", err)
	}
	if util.NodeDiscoveryEnabled() {
		if err := r.removeClusterRole(ctx, h, logger); err != nil {
			return fmt.Errorf("ClusterRole could not be removed: %w", err)
		}
		if err := r.removeClusterRoleBinding(ctx, h, logger); err != nil {
			return fmt.Errorf("ClusterRoleBinding could not be removed: %w", err)
		}
	}

	lk := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	r.statusServiceRegistry.Delete(lk)
	r.mtlsClientRegistry.Delete(lk.Namespace)
	if err := r.clientRegistry.Delete(ctx, lk); err != nil {
		return fmt.Errorf("Hazelcast client could not be deleted:  %w", err)
	}

	controllerutil.RemoveFinalizer(h, n.Finalizer)
	err := r.Update(ctx, h)
	if err != nil {
		return fmt.Errorf("failed to remove finalizer from custom resource: %w", err)
	}
	return nil
}

func (r *HazelcastReconciler) deleteDependentCRs(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {

	dependentCRs := map[string]client.ObjectList{
		"Map":               &hazelcastv1alpha1.MapList{},
		"MultiMap":          &hazelcastv1alpha1.MultiMapList{},
		"Topic":             &hazelcastv1alpha1.TopicList{},
		"ReplicatedMap":     &hazelcastv1alpha1.ReplicatedMapList{},
		"Queue":             &hazelcastv1alpha1.QueueList{},
		"Cache":             &hazelcastv1alpha1.CacheList{},
		"JetJob":            &hazelcastv1alpha1.JetJobList{},
		"UserCodeNamespace": &hazelcastv1alpha1.UserCodeNamespaceList{},
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

	dsItems := objList.(hazelcastv1alpha1.CRLister).GetItems()
	if len(dsItems) == 0 {
		return nil
	}

	g, groupCtx := errgroup.WithContext(ctx)
	for i := 0; i < len(dsItems); i++ {
		i := i
		g.Go(func() error {
			if dsItems[i].GetDeletionTimestamp() == nil {
				return util.DeleteObject(groupCtx, r.Client, dsItems[i])
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error deleting %v resources %w", crKind, err)
	}

	if err := r.Client.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return fmt.Errorf("hazelcast dependent %v resources are not deleted yet %w", crKind, err)
	}

	dsItems = objList.(hazelcastv1alpha1.CRLister).GetItems()
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
			Name:        h.ClusterScopedName(),
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get"},
			},
		}
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
			Name:        h.Name,
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	err := controllerutil.SetControllerReference(h, role, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "pods", "services"},
				Verbs:     []string{"get", "list"},
			},
		}
		if h.Spec.Persistence.IsEnabled() {
			role.Rules = append(role.Rules, rbacv1.PolicyRule{
				APIGroups: []string{"apps"},
				Resources: []string{"statefulsets"},
				Verbs:     []string{"watch", "list"},
			})
		}
		return nil
	})

	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Role", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) reconcileServiceAccount(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	// do not create SA if user specified reference to ServiceAccountName
	if h.Spec.ServiceAccountName != "" {
		return nil
	}

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceAccountName(h),
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	err := controllerutil.SetControllerReference(h, serviceAccount, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on ServiceAccount: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, serviceAccount, func() error {
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
			Name:        csName,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, crb, func() error {
		crb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName(h),
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
			Name:        h.Name,
			Namespace:   h.Namespace,
			Labels:      labels(h),
			Annotations: h.Spec.Annotations,
		},
	}

	err := controllerutil.SetControllerReference(h, rb, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, rb, func() error {
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName(h),
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
			Selector: util.Labels(h),
		},
	}

	if serviceType(h) == corev1.ServiceTypeClusterIP {
		// We want to use headless to be compatible with Hazelcast helm chart
		service.Spec.ClusterIP = "None"
	}

	err := controllerutil.SetControllerReference(h, service, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Service: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, service, func() error {
		if h.Spec.ExposeExternally.IsEnabled() {
			switch h.Spec.ExposeExternally.DiscoveryK8ServiceType() {
			case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
				service.Labels[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeDiscoveryLabelValue
			default:
				delete(service.Labels, n.ServiceEndpointTypeLabelName)
			}
		} else {
			delete(service.Labels, n.ServiceEndpointTypeLabelName)
		}

		// append default wan port to HZ Discovery Service if use did not configure
		isAddWANPort := false
		if h.Spec.AdvancedNetwork == nil || len(h.Spec.AdvancedNetwork.WAN) == 0 {
			isAddWANPort = true
		}

		service.Spec.Type = serviceType(h)
		service.Spec.Ports = util.EnrichServiceNodePorts(hazelcastPort(isAddWANPort), service.Spec.Ports)

		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Service", h.Name, "result", opResult)
	}

	return err
}

func (r *HazelcastReconciler) reconcileWANServices(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if h.Spec.AdvancedNetwork == nil {
		return nil
	}
	for _, w := range h.Spec.AdvancedNetwork.WAN {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        h.Name + "-" + w.Name,
				Namespace:   h.Namespace,
				Labels:      labels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector: util.Labels(h),
			},
		}

		err := controllerutil.SetControllerReference(h, service, r.Scheme)
		if err != nil {
			return err
		}

		var i uint
		var ports []corev1.ServicePort
		for i = 0; i < w.PortCount; i++ {
			ports = append(ports,
				corev1.ServicePort{
					Name:        fmt.Sprintf("%s%s-%d", n.WanPortNamePrefix, w.Name, i),
					Protocol:    corev1.ProtocolTCP,
					Port:        int32(w.Port + i),
					TargetPort:  intstr.FromInt(int(w.Port + i)),
					AppProtocol: pointer.String("tcp"),
				})
		}

		opResult, err := util.CreateOrUpdate(ctx, r.Client, service, func() error {
			switch w.ServiceType {
			case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
				service.Labels[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeWANLabelValue
			case corev1.ServiceTypeClusterIP:
				delete(service.Labels, n.ServiceEndpointTypeLabelName)
			}

			service.Spec.Ports = util.EnrichServiceNodePorts(ports, service.Spec.Ports)
			if w.ServiceType == "" {
				service.Spec.Type = v1.ServiceTypeLoadBalancer
			} else {
				service.Spec.Type = w.ServiceType
			}
			return nil
		})
		if opResult != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Service", h.Name, "result", opResult)
		}
		if err != nil {
			return err
		}
	}

	return nil
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
				Name:        servicePerPodName(i, h),
				Namespace:   h.Namespace,
				Labels:      servicePerPodLabels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector:                 servicePerPodSelector(i, h),
				PublishNotReadyAddresses: true,
			},
		}

		err := controllerutil.SetControllerReference(h, service, r.Scheme)
		if err != nil {
			return err
		}

		opResult, err := util.CreateOrUpdateForce(ctx, r.Client, service, func() error {
			switch h.Spec.ExposeExternally.MemberAccessServiceType() {
			case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
				service.Labels[n.ServiceEndpointTypeLabelName] = n.ServiceEndpointTypeMemberLabelValue
			default:
				delete(service.Labels, n.ServiceEndpointTypeLabelName)
			}

			service.Spec.Ports = util.EnrichServiceNodePorts([]corev1.ServicePort{clientPort()}, service.Spec.Ports)
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

// nodePublicAddress tries to find node public ip
func nodePublicAddress(addresses []v1.NodeAddress) string {
	var fallbackAddress string
	// we iterate over a unordered list of addresses
	for _, address := range addresses {
		switch address.Type {
		case corev1.NodeExternalIP:
			// we found explicitly set NodeExternalIP, fast return
			return address.Address
		case corev1.NodeInternalIP:
			fallbackAddress = address.Address
		}
	}
	// no NodeExternalIP found on the list so return fallback ip
	return fallbackAddress
}

func (r *HazelcastReconciler) reconcileHazelcastEndpoints(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	// prepare a map of node addresses for fast lookup
	var nodes corev1.NodeList
	nodeAddress := make(map[string]string, len(nodes.Items))
	if util.NodeDiscoveryEnabled() {
		if err := r.Client.List(ctx, &nodes); err != nil {
			return err
		}
		for _, node := range nodes.Items {
			nodeAddress[node.Name] = nodePublicAddress(node.Status.Addresses)
		}
	}

	svcList, err := util.ListRelatedServices(ctx, r.Client, h)
	if err != nil {
		return err
	}

	reconciledHzEndpointNames := make(map[string]any)

	for _, svc := range svcList.Items {
		endpointType, ok := svc.Labels[n.ServiceEndpointTypeLabelName]
		if !ok {
			continue
		}

		var hzEndpoints []*hazelcastv1alpha1.HazelcastEndpoint

		switch endpointType {
		case n.ServiceEndpointTypeDiscoveryLabelValue:
			for _, port := range svc.Spec.Ports {
				endpointNn := types.NamespacedName{
					Name:      svc.Name,
					Namespace: svc.Namespace,
				}
				// For the default Wan port when the WANConfig is not configured under the AdvancedNetwork config
				if port.Name == n.WanDefaultPortName {
					endpointNn.Name = fmt.Sprintf("%s-%s", endpointNn.Name, "wan")
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
					continue
				}
				if port.Name == n.HazelcastPortName {
					hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeDiscovery, port.Port))
					continue
				}
			}
		case n.ServiceEndpointTypeMemberLabelValue:
			endpointNn := types.NamespacedName{
				Name:      svc.Name,
				Namespace: svc.Namespace,
			}
			hzEndpoints = []*hazelcastv1alpha1.HazelcastEndpoint{
				hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeMember, clientPort().Port),
			}
		case n.ServiceEndpointTypeWANLabelValue:
			for i, port := range svc.Spec.Ports {
				endpointNn := types.NamespacedName{
					Name:      svc.Name,
					Namespace: svc.Namespace,
				}
				if len(svc.Spec.Ports) > 1 {
					endpointNn.Name = fmt.Sprintf("%s-%d", endpointNn.Name, i)
				}

				hzEndpoints = append(hzEndpoints, hazelcastEndpointFromService(endpointNn, h, hazelcastv1alpha1.HazelcastEndpointTypeWAN, port.Port))
			}
		default:
			return fmt.Errorf("service endpoint type label values '%s' is not matched", endpointType)
		}

		for _, hzEndpoint := range hzEndpoints {
			reconciledHzEndpointNames[hzEndpoint.Name] = struct{}{}

			err := controllerutil.SetOwnerReference(&svc, hzEndpoint, r.Scheme)
			if err != nil {
				return err
			}

			opResult, err := util.CreateOrUpdateForce(ctx, r.Client, hzEndpoint, func() error {
				return nil
			})
			if opResult != controllerutil.OperationResultNone {
				logger.Info("Operation result", "HazelcastEndpoint", hzEndpoint.Name, "result", opResult)
			}
			if err != nil {
				return err
			}

			// set external address depending on parent service type
			switch svc.Spec.Type {
			case corev1.ServiceTypeNodePort:
				// search for the first node ip of the first pod
				var pods corev1.PodList
				if err := r.Client.List(ctx, &pods, client.MatchingLabels(svc.Spec.Selector)); err != nil {
					return err
				}

				address := "*"
				if len(pods.Items) > 0 {
					address = nodeAddress[pods.Items[0].Spec.NodeName]
				}

				// depending on Endpoint type we get name of the port
				var portName string
				switch hzEndpoint.Spec.Type {
				case v1alpha1.HazelcastEndpointTypeWAN:
					portName = n.WanDefaultPortName
				default:
					portName = n.HazelcastPortName
				}

				// NodePorts get address from svc .nodePort property
				for _, port := range svc.Spec.Ports {
					if port.Name == portName {
						hzEndpoint.Status.Address = fmt.Sprintf("%s:%d", address, port.NodePort)
						break
					}
				}

			case corev1.ServiceTypeLoadBalancer:
				// LoadBalancers get address from ingress status property
				hzEndpoint.SetAddress(util.GetExternalAddress(&svc))
			}

			err = r.Client.Status().Update(ctx, hzEndpoint)
			if err != nil {
				return err
			}
		}
	}

	// Delete the leftover HazelcastEndpoints if any.
	// The leftover resources take place after disabling the exposeExternally as an update.
	hzEndpointList := hazelcastv1alpha1.HazelcastEndpointList{}
	nsOpt := client.InNamespace(h.Namespace)
	lblOpt := client.MatchingLabels(util.Labels(h))
	if err := r.Client.List(ctx, &hzEndpointList, nsOpt, lblOpt); err != nil {
		return err
	}
	for _, hzEndpoint := range hzEndpointList.Items {
		if _, ok := reconciledHzEndpointNames[hzEndpoint.Name]; !ok {
			logger.Info("Deleting leftover HazelcastEndpoint", "name", hzEndpoint.Name)
			err := r.Client.Delete(ctx, &hzEndpoint)
			if err != nil {
				return err
			}
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
	ls := util.Labels(h)
	ls[n.PodNameLabel] = servicePerPodName(i, h)
	return ls
}

func servicePerPodLabels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	ls := labels(h)
	ls[n.ServicePerPodLabelName] = n.LabelValueTrue
	return ls
}

func hazelcastPort(isAddWANPort bool) []v1.ServicePort {
	p := []corev1.ServicePort{
		clientPort(),
		{
			Name:        n.MemberPortName,
			Protocol:    v1.ProtocolTCP,
			Port:        n.MemberServerSocketPort,
			TargetPort:  intstr.FromInt(n.MemberServerSocketPort),
			AppProtocol: pointer.String("tcp"),
		},
		{
			Name:        n.RestPortName,
			Protocol:    v1.ProtocolTCP,
			Port:        n.RestServerSocketPort,
			TargetPort:  intstr.FromInt(n.RestServerSocketPort),
			AppProtocol: pointer.String("tcp"),
		},
	}

	if isAddWANPort {
		p = append(p, defaultWANPort())
	}

	return p
}

func clientPort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:        n.HazelcastPortName,
		Port:        n.DefaultHzPort,
		Protocol:    corev1.ProtocolTCP,
		TargetPort:  intstr.FromString(n.Hazelcast),
		AppProtocol: pointer.String("tcp"),
	}
}

func defaultWANPort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:        n.WanDefaultPortName,
		Protocol:    corev1.ProtocolTCP,
		AppProtocol: pointer.String("tcp"),
		Port:        n.WanDefaultPort,
		TargetPort:  intstr.FromInt(n.WanDefaultPort),
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

func hazelcastEndpointFromService(nn types.NamespacedName, hz *hazelcastv1alpha1.Hazelcast, endpointType hazelcastv1alpha1.HazelcastEndpointType, port int32) *hazelcastv1alpha1.HazelcastEndpoint {
	return &hazelcastv1alpha1.HazelcastEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nn.Name,
			Namespace:   nn.Namespace,
			Labels:      labels(hz),
			Annotations: hz.Spec.Annotations,
		},
		Spec: hazelcastv1alpha1.HazelcastEndpointSpec{
			Type:                  endpointType,
			Port:                  port,
			HazelcastResourceName: hz.Name,
		},
	}
}

func (r *HazelcastReconciler) reconcileInitContainerConfig(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metadata(h),
		Data:       make(map[string]string),
	}
	cm.Name = cm.Name + n.InitSuffix
	err := controllerutil.SetControllerReference(h, cm, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on ConfigMap: %w", err)
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
			cfg, err := initContainerConfig(ctx, r.Client, h, logger)
			if err != nil {
				return err
			}
			cm.Data[n.InitConfigFile] = string(cfg)
			return nil
		})
		if result != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Secret", h.Name, "result", result)
		}
		return err
	})
}

func (r *HazelcastReconciler) reconcileSecret(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	scrt := &corev1.Secret{
		ObjectMeta: metadata(h),
		Data:       make(map[string][]byte),
	}

	err := controllerutil.SetControllerReference(h, scrt, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Secret: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, scrt, func() error {
			config, err := hazelcastConfig(ctx, r.Client, h, logger)
			if err != nil {
				return err
			}
			scrt.Data["hazelcast.yaml"] = config

			if _, ok := scrt.Data["hazelcast.jks"]; !ok {
				keystore, err := hazelcastKeystore(ctx, r.Client, h)
				if err != nil {
					return err
				}
				scrt.Data["hazelcast.jks"] = keystore
			}

			return nil
		})
		if result != controllerutil.OperationResultNone {
			logger.Info("Operation result", "Secret", h.Name, "result", result)
		}
		return err
	})
}

func (r *HazelcastReconciler) reconcileMTLSSecret(ctx context.Context, h *hazelcastv1alpha1.Hazelcast) error {
	_, err := r.mtlsClientRegistry.Create(ctx, r.Client, h.Namespace)
	if err != nil {
		return err
	}
	secret := &v1.Secret{}
	secretName := types.NamespacedName{Name: n.MTLSCertSecretName, Namespace: h.Namespace}
	err = r.Client.Get(ctx, secretName, secret)
	if err != nil {
		return err
	}
	err = controllerutil.SetControllerReference(h, secret, r.Scheme)
	if err != nil {
		return err
	}
	_, err = util.CreateOrUpdateForce(ctx, r.Client, secret, func() error {
		return nil
	})
	return err
}

func initContainerConfig(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) ([]byte, error) {
	cfgW := compound.ConfigWrapper{
		InitContainer: &compound.Config{},
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		ns, err := filterUserCodeNamespaces(ctx, c, h)
		if err != nil {
			return nil, err
		}

		if len(ns) != 0 {
			buckets := make([]downloadbucket.Cmd, 0)
			for _, ucn := range ns {
				buckets = append(buckets, downloadbucket.Cmd{
					BucketURI:   ucn.Spec.BucketConfiguration.BucketURI,
					SecretName:  ucn.Spec.BucketConfiguration.SecretName,
					Destination: filepath.Join(n.UCNBucketPath, ucn.Name+".zip"),
				})
			}
			cfgW.InitContainer = &compound.Config{
				Download: &compound.Download{
					Bundle: &compound.Bundle{
						Buckets: buckets,
					},
				},
			}
		}
	}

	ucn := h.Spec.DeprecatedUserCodeDeployment

	if h.Spec.DeprecatedUserCodeDeployment.IsBucketEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			Buckets: []downloadbucket.Cmd{
				{
					Destination: n.UserCodeBucketPath,
					SecretName:  ucn.BucketConfiguration.GetSecretName(),
					BucketURI:   ucn.BucketConfiguration.BucketURI,
				},
			},
		}
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsRemoteURLsEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			URLs: []downloadurl.Cmd{
				{
					Destination: n.UserCodeBucketPath,
					URLs:        strings.Join(ucn.RemoteURLs, ","),
				},
			},
		}
	}

	jet := h.Spec.JetEngineConfiguration

	if h.Spec.JetEngineConfiguration.IsBucketEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			Buckets: []downloadbucket.Cmd{
				{
					Destination: n.JetJobJarsPath,
					SecretName:  jet.BucketConfiguration.GetSecretName(),
					BucketURI:   jet.BucketConfiguration.BucketURI,
				},
			},
		}
	}

	if h.Spec.JetEngineConfiguration.IsRemoteURLsEnabled() {
		cfgW.InitContainer.Download = &compound.Download{
			URLs: []downloadurl.Cmd{
				{
					Destination: n.JetJobJarsPath,
					URLs:        strings.Join(jet.RemoteURLs, ","),
				},
			},
		}
	}

	if !h.Spec.Persistence.IsRestoreEnabled() {
		return yaml.Marshal(cfgW)
	}

	hzConf, err := getHazelcastConfig(ctx, c, types.NamespacedName{Name: h.Name, Namespace: h.Namespace})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch existing HZ config: %v", err)
	}

	var r compound.Restore
	if h.Spec.Persistence.RestoreFromHotBackupResourceName() {
		r, err = restoreCmdForHBResource(ctx, c, h,
			types.NamespacedName{Namespace: h.Namespace, Name: h.Spec.Persistence.Restore.HotBackupResourceName}, hzConf.Hazelcast.Persistence.BaseDir)
		if err != nil {
			return nil, err
		}
	} else if h.Spec.Persistence.RestoreFromLocalBackup() {
		r = restoreLocalInitContainer(h, *h.Spec.Persistence.Restore.LocalConfiguration, hzConf.Hazelcast.Persistence.BaseDir)
	} else {
		r = restoreInitContainer(h, h.Spec.Persistence.Restore.BucketConfiguration.GetSecretName(),
			h.Spec.Persistence.Restore.BucketConfiguration.BucketURI, hzConf.Hazelcast.Persistence.BaseDir)
	}
	cfgW.InitContainer.Restore = &r

	return yaml.Marshal(cfgW)
}

func restoreCmdForHBResource(ctx context.Context, cl client.Client, h *hazelcastv1alpha1.Hazelcast, key types.NamespacedName, baseDir string) (compound.Restore, error) {
	hb := &hazelcastv1alpha1.HotBackup{}
	err := cl.Get(ctx, key, hb)
	if err != nil {
		return compound.Restore{}, err
	}

	if hb.Status.State != hazelcastv1alpha1.HotBackupSuccess {
		return compound.Restore{}, fmt.Errorf("restore hotbackup '%s' status is not %s", hb.Name, hazelcastv1alpha1.HotBackupSuccess)
	}

	var r compound.Restore
	if hb.Spec.IsExternal() {
		bucketURI := hb.Status.GetBucketURI()
		r = restoreInitContainer(h, hb.Spec.GetSecretName(), bucketURI, baseDir)
	} else {
		backupFolder := hb.Status.GetBackupFolder()
		r = restoreLocalInitContainer(h, hazelcastv1alpha1.RestoreFromLocalConfiguration{
			BackupFolder: backupFolder,
		}, baseDir)
	}

	return r, nil
}

func restoreInitContainer(h *hazelcastv1alpha1.Hazelcast, secretName, bucket, baseDir string) compound.Restore {
	return compound.Restore{
		Bucket: &restore.BucketToPVCCmd{
			Bucket:      bucket,
			Destination: baseDir,
			// Hostname:    "",
			SecretName: secretName,
			RestoreID:  h.Spec.Persistence.Restore.Hash(),
		},
	}
}

func restoreLocalInitContainer(h *hazelcastv1alpha1.Hazelcast, conf hazelcastv1alpha1.RestoreFromLocalConfiguration, destBaseDir string) compound.Restore {
	// it maybe configured by restore.localConfig.
	// HotBackup CR does not configure it. For HotBackup CR source and destination base directories are always the same.
	srcBaseDir := destBaseDir
	if conf.BaseDir != "" {
		srcBaseDir = conf.BaseDir
	}

	backupDir := n.BackupDir
	if conf.BackupDir != "" {
		backupDir = conf.BackupDir
	}

	backupFolder := conf.BackupFolder

	return compound.Restore{
		PVC: &restore.LocalInPVCCmd{
			BackupSequenceFolderName: backupFolder,
			BackupSourceBaseDir:      srcBaseDir,
			BackupDestinationBaseDir: destBaseDir,
			BackupDir:                backupDir,
			// Hostname:                 "",
			RestoreID: h.Spec.Persistence.Restore.Hash(),
		},
	}
}

func hazelcastConfig(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) ([]byte, error) {
	cfg := hazelcastBasicConfig(h)

	// For backward compatibility purpose; if the BaseDir is not equal to the expected, keep set it equal to MountPath
	if h.Spec.Persistence.IsEnabled() {
		existingConfig, err := getHazelcastConfig(ctx, c, types.NamespacedName{Name: h.Name, Namespace: h.Namespace})
		if err != nil {
			logger.V(util.WarnLevel).Info(fmt.Sprintf("failed to fetch old config secret %v", err))
		}
		if existingConfig != nil && existingConfig.Hazelcast.Persistence.BaseDir != n.BaseDir {
			cfg.Persistence.BaseDir = n.PersistenceMountPath
			cfg.Persistence.BackupDir = path.Join(n.PersistenceMountPath, "hot-backup")
		}
	}

	if h.Spec.Properties != nil {
		h.Spec.Properties = mergeProperties(logger, h.Spec.Properties)
	} else {
		h.Spec.Properties = make(map[string]string)
		for k, v := range DefaultProperties {
			h.Spec.Properties[k] = v
		}
	}
	// Temp solution for Tiered Storage provided by the core team.
	// To enable dynamic TS maps, the startup condition enabling TS service had to be changed.
	// Prior to this change, the service got initialized only if a TS map was present in the static configuration.
	// This forced our cloud offering to define a "fake" TS map in the static configuration.
	// Starting with this change, the TS service can be initialized with -Dhazelcast.tiered.store.force.enabled=true.
	if h.Spec.IsTieredStorageEnabled() {
		h.Spec.Properties["hazelcast.tiered.store.force.enabled"] = "true"
	}

	fillHazelcastConfigWithProperties(&cfg, h)
	fillHazelcastConfigWithExecutorServices(&cfg, h)
	fillHazelcastConfigWithSerialization(&cfg, h)

	ml, err := filterPersistedMaps(ctx, c, h)
	if err != nil {
		return nil, err
	}
	if err := fillHazelcastConfigWithMaps(ctx, c, &cfg, h, ml); err != nil {
		return nil, err
	}

	wrl, err := filterPersistedWanReplications(ctx, c, h)
	if err != nil {
		return nil, err
	}
	fillHazelcastConfigWithWanReplications(&cfg, wrl)

	ucn, err := filterUserCodeNamespaces(ctx, c, h)
	if err != nil {
		return nil, err
	}
	fillHazelcastConfigWithUserCodeNamespaces(&cfg, h, ucn)

	dataStructures := []client.ObjectList{
		&hazelcastv1alpha1.MultiMapList{},
		&hazelcastv1alpha1.TopicList{},
		&hazelcastv1alpha1.ReplicatedMapList{},
		&hazelcastv1alpha1.QueueList{},
		&hazelcastv1alpha1.CacheList{},
	}
	for _, ds := range dataStructures {
		filteredDSList, err := filterPersistedDS(ctx, c, h, ds)
		if err != nil {
			return nil, err
		}
		if len(filteredDSList) == 0 {
			continue
		}
		switch hazelcastv1alpha1.GetKind(filteredDSList[0]) {
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

	if h.Spec.CustomConfigCmName != "" {
		cfgCm := &v1.ConfigMap{}
		if err := c.Get(ctx, types.NamespacedName{Name: h.Spec.CustomConfigCmName, Namespace: h.Namespace}, cfgCm, nil); err != nil {
			return nil, err
		}
		var overwrite bool
		annotations := cfgCm.GetAnnotations()
		if v := annotations[n.HazelcastCustomConfigOverwriteAnnotation]; v == "true" {
			overwrite = true
		}
		cfgMap := make(map[string]interface{})
		if err = yaml.Unmarshal([]byte(cfgCm.Data[n.HazelcastCustomConfigKey]), cfgMap); err != nil {
			return nil, err
		}
		cfgMap, err = mergeConfig(cfgMap, &cfg, logger, overwrite)
		if err != nil {
			return nil, err
		}
		hzWrapper := make(map[string]interface{})
		hzWrapper["hazelcast"] = cfgMap
		return yaml.Marshal(hzWrapper)
	}

	return yaml.Marshal(config.HazelcastWrapper{Hazelcast: cfg})
}

func mergeProperties(logger logr.Logger, inputProps map[string]string) map[string]string {
	m := make(map[string]string)
	for k, v := range DefaultProperties {
		m[k] = v
	}
	for k, v := range inputProps {
		// if the property provided by the user is one of the default (immutable) property,
		// ignore the provided property without overriding the default one.
		if existingVal, exist := m[k]; exist {
			if v != existingVal {
				logger.V(util.DebugLevel).Info("Property ignored", "property", k, "value", v)
			}
		} else {
			m[k] = v
		}
	}
	return m
}

func mergeConfig(cstCfg map[string]interface{}, cfg *config.Hazelcast, logger logr.Logger, overwrite bool) (map[string]interface{}, error) {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	crCfg := make(map[string]interface{})
	if err = yaml.Unmarshal(out, crCfg); err != nil {
		return nil, err
	}
	if overwrite {
		deepMerge(crCfg, cstCfg)
		return crCfg, nil
	} else {
		for k, v := range crCfg {
			if _, exist := cstCfg[k]; exist {
				logger.V(util.WarnLevel).Info("Custom Config section ignored", "section", k)
			}
			cstCfg[k] = v
		}
	}
	return cstCfg, nil
}

func deepMerge(dst, src map[string]any) {
	for k := range src {
		// new section, fast copy
		if _, ok := dst[k]; !ok {
			dst[k] = src[k]
			continue
		}

		// merge maps
		if src2, ok := src[k].(map[string]any); ok {
			if dst2, ok := dst[k].(map[string]any); ok {
				deepMerge(dst2, src2)
				continue
			}
		}

		// add or overwrite value
		dst[k] = src[k]
	}
}

func hazelcastBasicConfig(h *hazelcastv1alpha1.Hazelcast) config.Hazelcast {
	cfg := config.Hazelcast{
		AdvancedNetwork: config.AdvancedNetwork{
			Enabled: true,
			Join: config.Join{
				Kubernetes: config.Kubernetes{
					Enabled:                 pointer.Bool(true),
					ServiceName:             h.Name,
					ServicePort:             n.MemberServerSocketPort,
					ServicePerPodLabelName:  n.ServicePerPodLabelName,
					ServicePerPodLabelValue: n.LabelValueTrue,
				},
			},
		},
	}

	if h.Spec.DeprecatedUserCodeDeployment != nil {
		cfg.UserCodeDeployment = config.UserCodeDeployment{
			Enabled: h.Spec.DeprecatedUserCodeDeployment.ClientEnabled,
		}
	}

	if h.Spec.JetEngineConfiguration.IsConfigured() {
		cfg.Jet = config.Jet{
			Enabled:               h.Spec.JetEngineConfiguration.Enabled,
			ResourceUploadEnabled: pointer.Bool(h.Spec.JetEngineConfiguration.ResourceUploadEnabled),
		}

		if h.Spec.JetEngineConfiguration.Instance.IsConfigured() {
			i := h.Spec.JetEngineConfiguration.Instance
			cfg.Jet.Instance = config.JetInstance{
				CooperativeThreadCount:         i.CooperativeThreadCount,
				FlowControlPeriodMillis:        &i.FlowControlPeriodMillis,
				BackupCount:                    &i.BackupCount,
				ScaleUpDelayMillis:             &i.ScaleUpDelayMillis,
				LosslessRestartEnabled:         &i.LosslessRestartEnabled,
				MaxProcessorAccumulatedRecords: i.MaxProcessorAccumulatedRecords,
			}
		}

		if h.Spec.JetEngineConfiguration.EdgeDefaults.IsConfigured() {
			e := h.Spec.JetEngineConfiguration.EdgeDefaults
			cfg.Jet.EdgeDefaults = config.EdgeDefaults{
				QueueSize:               e.QueueSize,
				PacketSizeLimit:         e.PacketSizeLimit,
				ReceiveWindowMultiplier: e.ReceiveWindowMultiplier,
			}
		}
	}

	if h.Spec.ExposeExternally.UsesNodeName() {
		cfg.AdvancedNetwork.Join.Kubernetes.UseNodeNameAsExternalAddress = pointer.Bool(true)
	}

	if h.Spec.ClusterName != "" {
		cfg.ClusterName = h.Spec.ClusterName
	}

	if h.Spec.Persistence.IsEnabled() {
		cfg.Persistence = config.Persistence{
			Enabled:                   pointer.Bool(true),
			BaseDir:                   n.BaseDir,
			BackupDir:                 path.Join(n.BaseDir, "hot-backup"),
			Parallelism:               1,
			ValidationTimeoutSec:      120,
			ClusterDataRecoveryPolicy: clusterDataRecoveryPolicy(h.Spec.Persistence.ClusterDataRecoveryPolicy),
			AutoRemoveStaleData:       pointer.Bool(h.Spec.Persistence.AutoRemoveStaleData()),
		}
		if h.Spec.Persistence.DataRecoveryTimeout != 0 {
			cfg.Persistence.ValidationTimeoutSec = h.Spec.Persistence.DataRecoveryTimeout
		}
	}

	if h.Spec.HighAvailabilityMode != "" {
		switch h.Spec.HighAvailabilityMode {
		case hazelcastv1alpha1.HighAvailabilityNodeMode:
			cfg.PartitionGroup = config.PartitionGroup{
				Enabled:   pointer.Bool(true),
				GroupType: "NODE_AWARE",
			}
		case hazelcastv1alpha1.HighAvailabilityZoneMode:
			cfg.PartitionGroup = config.PartitionGroup{
				Enabled:   pointer.Bool(true),
				GroupType: "ZONE_AWARE",
			}
		}
	}

	if h.Spec.NativeMemory.IsEnabled() {
		nativeMemory := h.Spec.NativeMemory
		cfg.NativeMemory = config.NativeMemory{
			Enabled:                 true,
			AllocatorType:           string(nativeMemory.AllocatorType),
			MinBlockSize:            nativeMemory.MinBlockSize,
			PageSize:                nativeMemory.PageSize,
			MetadataSpacePercentage: nativeMemory.MetadataSpacePercentage,
			Size: config.Size{
				Value: nativeMemory.Size.ScaledValue(resource.Mega),
				Unit:  "MEGABYTES",
			},
		}
	}

	// Member Network
	cfg.AdvancedNetwork.MemberServerSocketEndpointConfig = config.MemberServerSocketEndpointConfig{
		Port: config.PortAndPortCount{
			Port:      n.MemberServerSocketPort,
			PortCount: 1,
		},
	}

	if h.Spec.AdvancedNetwork != nil && len(h.Spec.AdvancedNetwork.MemberServerSocketEndpointConfig.Interfaces) != 0 {
		cfg.AdvancedNetwork.MemberServerSocketEndpointConfig.Interfaces = config.Interfaces{
			Enabled:    true,
			Interfaces: h.Spec.AdvancedNetwork.MemberServerSocketEndpointConfig.Interfaces,
		}
	}

	// Client Network
	cfg.AdvancedNetwork.ClientServerSocketEndpointConfig = config.ClientServerSocketEndpointConfig{
		Port: config.PortAndPortCount{
			Port:      n.ClientServerSocketPort,
			PortCount: 1,
		},
	}

	if h.Spec.AdvancedNetwork != nil && len(h.Spec.AdvancedNetwork.ClientServerSocketEndpointConfig.Interfaces) != 0 {
		cfg.AdvancedNetwork.ClientServerSocketEndpointConfig.Interfaces = config.Interfaces{
			Enabled:    true,
			Interfaces: h.Spec.AdvancedNetwork.ClientServerSocketEndpointConfig.Interfaces,
		}
	}

	// Rest Network
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.Port = config.PortAndPortCount{
		Port:      n.RestServerSocketPort,
		PortCount: 1,
	}
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.EndpointGroups.Persistence.Enabled = pointer.Bool(true)
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.EndpointGroups.HealthCheck.Enabled = pointer.Bool(true)
	cfg.AdvancedNetwork.RestServerSocketEndpointConfig.EndpointGroups.ClusterWrite.Enabled = pointer.Bool(true)

	// WAN Network
	if h.Spec.AdvancedNetwork != nil && len(h.Spec.AdvancedNetwork.WAN) > 0 {
		cfg.AdvancedNetwork.WanServerSocketEndpointConfig = make(map[string]config.WanPort)
		for _, w := range h.Spec.AdvancedNetwork.WAN {
			cfg.AdvancedNetwork.WanServerSocketEndpointConfig[w.Name] = config.WanPort{
				PortAndPortCount: config.PortAndPortCount{
					Port:      w.Port,
					PortCount: w.PortCount,
				},
			}
		}
	} else { //Default WAN Configuration
		cfg.AdvancedNetwork.WanServerSocketEndpointConfig = make(map[string]config.WanPort)
		cfg.AdvancedNetwork.WanServerSocketEndpointConfig["default"] = config.WanPort{
			PortAndPortCount: config.PortAndPortCount{
				Port:      n.WanDefaultPort,
				PortCount: 1,
			},
		}
	}

	if h.Spec.ManagementCenterConfig != nil {
		cfg.ManagementCenter = config.ManagementCenterConfig{
			ScriptingEnabled:  h.Spec.ManagementCenterConfig.ScriptingEnabled,
			ConsoleEnabled:    h.Spec.ManagementCenterConfig.ConsoleEnabled,
			DataAccessEnabled: h.Spec.ManagementCenterConfig.DataAccessEnabled,
		}
	}

	if h.Spec.TLS != nil && h.Spec.TLS.SecretName != "" {
		var (
			jksPath  = path.Join(n.HazelcastMountPath, "hazelcast.jks")
			password = "hazelcast"
		)
		// require MTLS for member-member communication
		cfg.AdvancedNetwork.MemberServerSocketEndpointConfig.SSL = config.SSL{
			Enabled:          pointer.Bool(true),
			FactoryClassName: "com.hazelcast.nio.ssl.BasicSSLContextFactory",
			Properties:       NewSSLProperties(jksPath, password, "TLS", v1alpha1.MutualAuthenticationRequired),
		}
		// for client-server configuration use only server TLS
		cfg.AdvancedNetwork.ClientServerSocketEndpointConfig.SSL = config.SSL{
			Enabled:          pointer.Bool(true),
			FactoryClassName: "com.hazelcast.nio.ssl.BasicSSLContextFactory",
			Properties:       NewSSLProperties(jksPath, password, "TLS", h.Spec.TLS.MutualAuthentication),
		}
	}

	if h.Spec.SQL != nil {
		cfg.SQL = config.SQL{
			StatementTimeout:   h.Spec.SQL.StatementTimeout,
			CatalogPersistence: h.Spec.SQL.CatalogPersistenceEnabled,
		}
	}

	if h.Spec.IsTieredStorageEnabled() {
		cfg.LocalDevice = map[string]config.LocalDevice{}
		for _, ld := range h.Spec.LocalDevices {
			cfg.LocalDevice[ld.Name] = createLocalDeviceConfig(ld)
		}
	}
	if h.Spec.CPSubsystem.IsEnabled() {
		cfg.CPSubsystem = config.CPSubsystem{
			CPMemberCount:                     *h.Spec.ClusterSize,
			PersistenceEnabled:                true,
			BaseDir:                           n.CPBaseDir,
			GroupSize:                         h.Spec.ClusterSize,
			SessionTimeToLiveSeconds:          h.Spec.CPSubsystem.SessionTTLSeconds,
			SessionHeartbeatIntervalSeconds:   h.Spec.CPSubsystem.SessionHeartbeatIntervalSeconds,
			MissingCpMemberAutoRemovalSeconds: h.Spec.CPSubsystem.MissingCpMemberAutoRemovalSeconds,
			FailOnIndeterminateOperationState: h.Spec.CPSubsystem.FailOnIndeterminateOperationState,
			DataLoadTimeoutSeconds:            h.Spec.CPSubsystem.DataLoadTimeoutSeconds,
		}
		if h.Spec.Persistence.IsEnabled() && !h.Spec.CPSubsystem.IsPVC() {
			cfg.CPSubsystem.BaseDir = n.PersistenceMountPath + n.CPDirSuffix
		}
	}

	return cfg
}

func NewSSLProperties(path, password, protocol string, auth v1alpha1.MutualAuthentication) config.SSLProperties {
	const typ = "JKS"
	switch auth {
	case v1alpha1.MutualAuthenticationRequired:
		return config.SSLProperties{
			Protocol:             protocol,
			MutualAuthentication: "REQUIRED",
			// server cert + key
			KeyStoreType:     typ,
			KeyStore:         path,
			KeyStorePassword: password,
			// trusted cert pool (we use the same file for convince)
			TrustStoreType:     typ,
			TrustStore:         path,
			TrustStorePassword: password,
		}
	case v1alpha1.MutualAuthenticationOptional:
		return config.SSLProperties{
			Protocol:             protocol,
			MutualAuthentication: "OPTIONAL",
			KeyStoreType:         typ,
			KeyStore:             path,
			KeyStorePassword:     password,
			TrustStoreType:       typ,
			TrustStore:           path,
			TrustStorePassword:   password,
		}
	default:
		return config.SSLProperties{
			Protocol:         protocol,
			KeyStoreType:     typ,
			KeyStore:         path,
			KeyStorePassword: password,
		}
	}
}

func hazelcastKeystore(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) ([]byte, error) {
	var (
		store    = keystore.New()
		password = []byte("hazelcast")
	)
	if h.Spec.TLS != nil && h.Spec.TLS.SecretName != "" {
		cert, key, err := loadTLSKeyPair(ctx, c, h)
		if err != nil {
			return nil, err
		}
		err = store.SetPrivateKeyEntry("hazelcast", keystore.PrivateKeyEntry{
			CreationTime: time.Now(),
			PrivateKey:   key,
			CertificateChain: []keystore.Certificate{{
				Type:    "X509",
				Content: cert,
			}},
		}, password)
		if err != nil {
			return nil, err
		}
	}
	var b bytes.Buffer
	if err := store.Store(&b, password); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func loadTLSKeyPair(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) (cert []byte, key []byte, err error) {
	var s v1.Secret
	err = c.Get(ctx, types.NamespacedName{Name: h.Spec.TLS.SecretName, Namespace: h.Namespace}, &s)
	if err != nil {
		return
	}
	cert, err = decodePEM(s.Data["tls.crt"], "CERTIFICATE")
	if err != nil {
		return
	}
	key, err = decodePEM(s.Data["tls.key"], "PRIVATE KEY")
	if err != nil {
		return
	}
	return
}

func decodePEM(data []byte, typ string) ([]byte, error) {
	b, _ := pem.Decode(data)
	if b == nil {
		return nil, fmt.Errorf("expected at least one pem block")
	}
	if b.Type != typ {
		return nil, fmt.Errorf("expected type %v, got %v", typ, b.Type)
	}
	return b.Bytes, nil
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

func filterPersistedMaps(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) ([]hazelcastv1alpha1.Map, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	mapList := &hazelcastv1alpha1.MapList{}
	if err := c.List(ctx, mapList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}

	l := make([]hazelcastv1alpha1.Map, 0)

	for _, mp := range mapList.Items {
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
	return l, nil
}

func filterPersistedDS(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, objList client.ObjectList) ([]client.Object, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	if err := c.List(ctx, objList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}
	l := make([]client.Object, 0)
	for _, obj := range objList.(hazelcastv1alpha1.CRLister).GetItems() {
		if isDSPersisted(obj) {
			l = append(l, obj)
		}
	}
	return l, nil
}

func filterPersistedWanReplications(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) (map[string][]hazelcastv1alpha1.WanReplication, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	wrList := &hazelcastv1alpha1.WanReplicationList{}
	if err := c.List(ctx, wrList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}

	l := make(map[string][]hazelcastv1alpha1.WanReplication, 0)
	for _, wr := range wrList.Items {
		for wanKey, mapStatus := range wr.Status.WanReplicationMapsStatus {
			hzName, _ := splitWanMapKey(wanKey)
			if hzName != h.Name {
				continue
			}
			switch mapStatus.Status {
			case hazelcastv1alpha1.WanStatusPersisting, hazelcastv1alpha1.WanStatusSuccess:
				if l[wanKey] == nil {
					l[wanKey] = make([]hazelcastv1alpha1.WanReplication, 0)
				}
				l[wanKey] = append(l[wanKey], wr)
			default: // TODO, might want to do something for the other cases
			}
		}

	}
	return l, nil
}

func filterUserCodeNamespaces(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) ([]hazelcastv1alpha1.UserCodeNamespace, error) {
	fieldMatcher := client.MatchingFields{"hazelcastResourceName": h.Name}
	nsMatcher := client.InNamespace(h.Namespace)

	ucnList := &hazelcastv1alpha1.UserCodeNamespaceList{}

	if err := c.List(ctx, ucnList, fieldMatcher, nsMatcher); err != nil {
		return nil, err
	}

	l := make([]hazelcastv1alpha1.UserCodeNamespace, 0)

	for _, ucn := range ucnList.Items {
		switch ucn.Status.State {
		case hazelcastv1alpha1.UserCodeNamespaceSuccess:
			l = append(l, ucn)
		case hazelcastv1alpha1.UserCodeNamespaceFailure, hazelcastv1alpha1.UserCodeNamespacePending:
			if spec, ok := ucn.Annotations[n.LastSuccessfulSpecAnnotation]; ok {
				ucns := &hazelcastv1alpha1.UserCodeNamespaceSpec{}
				err := json.Unmarshal([]byte(spec), ucns)
				if err != nil {
					continue
				}
				ucn.Spec = *ucns
				l = append(l, ucn)
			}
		default:
		}
	}
	return l, nil
}

type ucnResourceType string

const (
	jarsInZip ucnResourceType = "JARS_IN_ZIP"
	jarFile   ucnResourceType = "JAR"
	classFile ucnResourceType = "CLASS"
)

func fillHazelcastConfigWithUserCodeNamespaces(cfg *config.Hazelcast, h *hazelcastv1alpha1.Hazelcast, ucn []hazelcastv1alpha1.UserCodeNamespace) {
	if !h.Spec.UserCodeNamespaces.IsEnabled() {
		return
	}
	cfg.UserCodeNamespaces = config.UserCodeNamespaces{
		Enabled:    pointer.Bool(true),
		Namespaces: make(map[string][]config.UserCodeNamespaceResource, len(ucn)),
	}
	if h.Spec.UserCodeNamespaces.ClassFilter != nil {
		cfg.UserCodeNamespaces.ClassFilter = &config.JavaFilterConfig{
			Blacklist: filterList(h.Spec.UserCodeNamespaces.ClassFilter.Blacklist),
			Whitelist: filterList(h.Spec.UserCodeNamespaces.ClassFilter.Whitelist),
		}
	}
	for _, u := range ucn {
		cfg.UserCodeNamespaces.Namespaces[u.Name] = []config.UserCodeNamespaceResource{{
			ID:           "bundle",
			ResourceType: string(jarsInZip),
			URL:          "file://" + filepath.Join(n.UCNBucketPath, u.Name+".zip"),
		}}
	}
}

func fillHazelcastConfigWithProperties(cfg *config.Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	cfg.Properties = h.Spec.Properties
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

func fillHazelcastConfigWithWanReplications(cfg *config.Hazelcast, wrl map[string][]hazelcastv1alpha1.WanReplication) {
	if len(wrl) != 0 {
		cfg.WanReplication = map[string]config.WanReplicationConfig{}
		for wanKey, wan := range wrl {
			_, mapName := splitWanMapKey(wanKey)
			wanConfig := createWanReplicationConfig(wanKey, wan)
			cfg.WanReplication[wanName(mapName)] = wanConfig
		}
	}
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

func fillHazelcastConfigWithSerialization(cfg *config.Hazelcast, h *hazelcastv1alpha1.Hazelcast) {
	if h.Spec.Serialization == nil {
		return
	}
	s := h.Spec.Serialization
	byteOrder := "BIG_ENDIAN"
	if s.ByteOrder == hazelcastv1alpha1.LittleEndian {
		byteOrder = "LITTLE_ENDIAN"
	}
	cfg.Serialization = config.Serialization{
		UseNativeByteOrder:         s.ByteOrder == hazelcastv1alpha1.NativeByteOrder,
		ByteOrder:                  byteOrder,
		DataSerializableFactories:  factories(s.DataSerializableFactories),
		PortableFactories:          factories(s.PortableFactories),
		EnableCompression:          s.EnableCompression,
		EnableSharedObject:         s.EnableSharedObject,
		OverrideDefaultSerializers: s.OverrideDefaultSerializers,
		AllowUnsafe:                s.AllowUnsafe,
		Serializers:                serializers(s.Serializers),
	}
	if s.GlobalSerializer != nil {
		cfg.Serialization.GlobalSerializer = &config.GlobalSerializer{
			OverrideJavaSerialization: s.GlobalSerializer.OverrideJavaSerialization,
			ClassName:                 s.GlobalSerializer.ClassName,
		}
	}
	if s.JavaSerializationFilter != nil {
		cfg.Serialization.JavaSerializationFilter = &config.JavaFilterConfig{
			Blacklist: filterList(s.JavaSerializationFilter.Blacklist),
			Whitelist: filterList(s.JavaSerializationFilter.Whitelist),
		}
	}
	if s.CompactSerialization != nil {
		classes := make([]string, 0, len(s.CompactSerialization.Classes))
		for _, class := range s.CompactSerialization.Classes {
			classes = append(classes, fmt.Sprintf("class: %s", class))
		}
		serializers := make([]string, 0, len(s.CompactSerialization.Serializers))
		for _, serializer := range s.CompactSerialization.Serializers {
			serializers = append(serializers, fmt.Sprintf("serializer: %s", serializer))
		}
		cfg.Serialization.CompactSerialization = &config.CompactSerialization{
			Serializers: serializers,
			Classes:     classes,
		}
	}
}

func filterList(jsf *hazelcastv1alpha1.SerializationFilterList) *config.FilterList {
	if jsf == nil {
		return nil
	}
	return &config.FilterList{
		Classes:  jsf.Classes,
		Packages: jsf.Packages,
		Prefixes: jsf.Prefixes,
	}
}

func serializers(srs []hazelcastv1alpha1.Serializer) []config.Serializer {
	var res []config.Serializer
	for _, sr := range srs {
		res = append(res, config.Serializer{
			TypeClass: sr.TypeClass,
			ClassName: sr.ClassName,
		})
	}
	return res
}

func factories(factories []string) []config.ClassFactories {
	var classFactories []config.ClassFactories
	for i, f := range factories {
		classFactories = append(classFactories, config.ClassFactories{
			FactoryId: int32(i),
			ClassName: f,
		})
	}
	return classFactories
}

func createMapConfig(ctx context.Context, c client.Client, hz *hazelcastv1alpha1.Hazelcast, m *hazelcastv1alpha1.Map) (config.Map, error) {
	ms := m.Spec
	mc := config.Map{
		BackupCount:       *ms.BackupCount,
		AsyncBackupCount:  ms.AsyncBackupCount,
		TimeToLiveSeconds: ms.TimeToLiveSeconds,
		ReadBackupData:    false,
		InMemoryFormat:    string(ms.InMemoryFormat),
		Indexes:           copyMapIndexes(ms.Indexes),
		Attributes:        attributes(ms.Attributes),
		StatisticsEnabled: true,
		DataPersistence: config.DataPersistence{
			Enabled: ms.PersistenceEnabled,
			Fsync:   false,
		},
		Eviction: config.MapEviction{
			Size:           ms.Eviction.MaxSize,
			MaxSizePolicy:  string(ms.Eviction.MaxSizePolicy),
			EvictionPolicy: string(ms.Eviction.EvictionPolicy),
		},
		UserCodeNamespace: ms.UserCodeNamespace,
	}

	mc.WanReplicationReference = wanReplicationRef(defaultWanReplicationRefCodec(m))

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

	if ms.NearCache != nil {
		mc.NearCache.InMemoryFormat = string(ms.NearCache.InMemoryFormat)
		mc.NearCache.InvalidateOnChange = *ms.NearCache.InvalidateOnChange
		mc.NearCache.TimeToLiveSeconds = ms.NearCache.TimeToLiveSeconds
		mc.NearCache.MaxIdleSeconds = ms.NearCache.MaxIdleSeconds
		mc.NearCache.Eviction = config.NearCacheEviction{
			Size:           ms.NearCache.NearCacheEviction.Size,
			MaxSizePolicy:  string(ms.NearCache.NearCacheEviction.MaxSizePolicy),
			EvictionPolicy: string(ms.NearCache.NearCacheEviction.EvictionPolicy),
		}
		mc.NearCache.CacheLocalEntries = *ms.NearCache.CacheLocalEntries
	}

	if ms.EventJournal != nil {
		mc.EventJournal.Enabled = true
		mc.EventJournal.Capacity = ms.EventJournal.Capacity
		mc.EventJournal.TimeToLiveSeconds = ms.EventJournal.TimeToLiveSeconds
	}

	if ms.TieredStore != nil {
		mc.TieredStore.Enabled = true
		mc.TieredStore.MemoryTier.Capacity = config.Size{
			Value: ms.TieredStore.MemoryCapacity.Value(),
			Unit:  "BYTES",
		}
		mc.TieredStore.DiskTier.Enabled = true
		mc.TieredStore.DiskTier.DeviceName = ms.TieredStore.DiskDeviceName
	}

	if ms.MerkleTree != nil {
		mc.MerkleTree.Enabled = true
		mc.MerkleTree.Depth = ms.MerkleTree.Depth
	}

	return mc, nil
}

func attributes(attributes []v1alpha1.AttributeConfig) []config.Attribute {
	var att []config.Attribute

	for _, a := range attributes {
		att = append(att, config.Attribute{
			Name:               a.Name,
			ExtractorClassName: a.ExtractorClassName,
		})
	}

	return att
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
	return config.ExecutorService{
		PoolSize:          es.PoolSize,
		QueueCapacity:     es.QueueCapacity,
		UserCodeNamespace: es.UserCodeNamespace,
	}
}

func createDurableExecutorServiceConfig(des *hazelcastv1alpha1.DurableExecutorServiceConfiguration) config.DurableExecutorService {
	return config.DurableExecutorService{PoolSize: des.PoolSize,
		Durability:        des.Durability,
		Capacity:          des.Capacity,
		UserCodeNamespace: des.UserCodeNamespace,
	}
}

func createScheduledExecutorServiceConfig(ses *hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) config.ScheduledExecutorService {
	return config.ScheduledExecutorService{
		PoolSize:          ses.PoolSize,
		Durability:        ses.Durability,
		Capacity:          ses.Capacity,
		CapacityPolicy:    ses.CapacityPolicy,
		UserCodeNamespace: ses.UserCodeNamespace,
	}
}

func createMultiMapConfig(mm *hazelcastv1alpha1.MultiMap) config.MultiMap {
	mms := mm.Spec
	return config.MultiMap{
		BackupCount:       *mms.BackupCount,
		AsyncBackupCount:  mms.AsyncBackupCount,
		Binary:            mms.Binary,
		CollectionType:    string(mms.CollectionType),
		StatisticsEnabled: n.DefaultMultiMapStatisticsEnabled,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultMultiMapMergePolicy,
			BatchSize: n.DefaultMultiMapMergeBatchSize,
		},
		UserCodeNamespace: mm.Spec.UserCodeNamespace,
	}
}

func createQueueConfig(q *hazelcastv1alpha1.Queue) config.Queue {
	qs := q.Spec
	return config.Queue{
		BackupCount:             *qs.BackupCount,
		AsyncBackupCount:        qs.AsyncBackupCount,
		EmptyQueueTtl:           *qs.EmptyQueueTtlSeconds,
		MaxSize:                 qs.MaxSize,
		StatisticsEnabled:       n.DefaultQueueStatisticsEnabled,
		PriorityComparatorClass: qs.PriorityComparatorClassName,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultQueueMergePolicy,
			BatchSize: n.DefaultQueueMergeBatchSize,
		},
		UserCodeNamespace: q.Spec.UserCodeNamespace,
	}
}

func createCacheConfig(c *hazelcastv1alpha1.Cache) config.Cache {
	cs := c.Spec
	cache := config.Cache{
		BackupCount:       *cs.BackupCount,
		AsyncBackupCount:  cs.AsyncBackupCount,
		StatisticsEnabled: n.DefaultCacheStatisticsEnabled,
		ManagementEnabled: n.DefaultCacheManagementEnabled,
		ReadThrough:       n.DefaultCacheReadThrough,
		WriteThrough:      n.DefaultCacheWriteThrough,
		InMemoryFormat:    string(cs.InMemoryFormat),
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultCacheMergePolicy,
			BatchSize: n.DefaultCacheMergeBatchSize,
		},
		DataPersistence: config.DataPersistence{
			Enabled: cs.PersistenceEnabled,
			Fsync:   false,
		},
		UserCodeNamespace: c.Spec.UserCodeNamespace,
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
	if cs.EventJournal != nil {
		cache.EventJournal.Enabled = true
		cache.EventJournal.Capacity = cs.EventJournal.Capacity
		cache.EventJournal.TimeToLiveSeconds = cs.EventJournal.TimeToLiveSeconds
	}

	return cache
}

func createTopicConfig(t *hazelcastv1alpha1.Topic) config.Topic {
	ts := t.Spec
	return config.Topic{
		GlobalOrderingEnabled: ts.GlobalOrderingEnabled,
		MultiThreadingEnabled: ts.MultiThreadingEnabled,
		StatisticsEnabled:     n.DefaultTopicStatisticsEnabled,
		UserCodeNamespace:     t.Spec.UserCodeNamespace,
	}
}

func createReplicatedMapConfig(rm *hazelcastv1alpha1.ReplicatedMap) config.ReplicatedMap {
	rms := rm.Spec
	return config.ReplicatedMap{
		InMemoryFormat:    string(rms.InMemoryFormat),
		AsyncFillup:       *rms.AsyncFillup,
		StatisticsEnabled: n.DefaultReplicatedMapStatisticsEnabled,
		MergePolicy: config.MergePolicy{
			ClassName: n.DefaultReplicatedMapMergePolicy,
			BatchSize: n.DefaultReplicatedMapMergeBatchSize,
		},
		UserCodeNamespace: rm.Spec.UserCodeNamespace,
	}
}

func createWanReplicationConfig(wanKey string, wrs []hazelcastv1alpha1.WanReplication) config.WanReplicationConfig {
	cfg := config.WanReplicationConfig{
		BatchPublisher: make(map[string]config.BatchPublisherConfig),
	}
	if len(wrs) != 0 {
		for _, wr := range wrs {
			bpc := createBatchPublisherConfig(wr)
			cfg.BatchPublisher[wr.Status.WanReplicationMapsStatus[wanKey].PublisherId] = bpc
		}
	}
	return cfg
}

func createBatchPublisherConfig(wr hazelcastv1alpha1.WanReplication) config.BatchPublisherConfig {
	bpc := config.BatchPublisherConfig{
		ClusterName:           wr.Spec.TargetClusterName,
		TargetEndpoints:       wr.Spec.Endpoints,
		ResponseTimeoutMillis: wr.Spec.Acknowledgement.Timeout,
		AcknowledgementType:   string(wr.Spec.Acknowledgement.Type),
		QueueCapacity:         wr.Spec.Queue.Capacity,
		QueueFullBehavior:     string(wr.Spec.Queue.FullBehavior),
		BatchSize:             wr.Spec.Batch.Size,
		BatchMaxDelayMillis:   wr.Spec.Batch.MaximumDelay,
	}
	if wr.Spec.SyncConsistencyCheckStrategy != "" {
		bpc.Sync = &config.Sync{
			ConsistencyCheckStrategy: string(wr.Spec.SyncConsistencyCheckStrategy),
		}
	}
	return bpc
}

func createLocalDeviceConfig(ld hazelcastv1alpha1.LocalDeviceConfig) config.LocalDevice {
	return config.LocalDevice{
		BaseDir: path.Join(n.TieredStorageBaseDir, ld.Name),
		Capacity: config.Size{
			Value: ld.PVC.RequestStorage.Value(),
			Unit:  "BYTES",
		},
		BlockSize:          ld.BlockSize,
		ReadIOThreadCount:  ld.ReadIOThreadCount,
		WriteIOThreadCount: ld.WriteIOThreadCount,
	}
}

func (r *HazelcastReconciler) reconcileStatefulset(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metadata(h),
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: util.Labels(h),
			},
			ServiceName:         h.Name,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels(h),
					Annotations: h.Spec.Annotations,
				},
				Spec: v1.PodSpec{
					ServiceAccountName: serviceAccountName(h),
					SecurityContext:    podSecurityContext(),
					Containers: []v1.Container{{
						Name: n.Hazelcast,
						LivenessProbe: &v1.Probe{
							ProbeHandler: v1.ProbeHandler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/hazelcast/health/node-state",
									Port:   intstr.FromInt(n.RestServerSocketPort),
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
									Port:   intstr.FromInt(n.RestServerSocketPort),
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 0,
							TimeoutSeconds:      10,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    10,
						},
						SecurityContext: containerSecurityContext(),
					}},
					TerminationGracePeriodSeconds: pointer.Int64(600),
				},
			},
		},
	}

	pvcName := n.PVCName
	if h.Spec.Persistence.RestoreFromLocalBackup() {
		pvcName = string(h.Spec.Persistence.Restore.LocalConfiguration.PVCNamePrefix)
	}

	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, sidecarContainer(h))
	sts.Spec.VolumeClaimTemplates = persistentVolumeClaims(h, pvcName)
	if h.Spec.IsTieredStorageEnabled() {
		sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, localDevicePersistentVolumeClaim(h)...)
	}

	err := controllerutil.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set owner reference on Statefulset: %w", err)
	}

	opResult, err := util.CreateOrUpdateForce(ctx, r.Client, sts, func() error {
		if len(sts.Spec.VolumeClaimTemplates) > 0 {
			pvcName = sts.Spec.VolumeClaimTemplates[0].Name
		}
		sts.Spec.Replicas = h.Spec.ClusterSize
		sts.ObjectMeta.Annotations = statefulSetAnnotations(sts, h)
		sts.Spec.Template.Annotations, err = podAnnotations(sts.Spec.Template.Annotations, h)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.ServiceAccountName = serviceAccountName(h)
		sts.Spec.Template.Spec.ImagePullSecrets = h.Spec.ImagePullSecrets
		sts.Spec.Template.Spec.Containers[0].Image = h.DockerImage()
		sts.Spec.Template.Spec.Containers[0].Env = env(h)
		sts.Spec.Template.Spec.Containers[0].ImagePullPolicy = h.Spec.ImagePullPolicy
		if h.Spec.Resources != nil {
			sts.Spec.Template.Spec.Containers[0].Resources = *h.Spec.Resources
		}
		sts.Spec.Template.Spec.Containers[0].Ports = hazelcastContainerPorts(h)

		if h.Spec.Scheduling != nil {
			sts.Spec.Template.Spec.Affinity = h.Spec.Scheduling.Affinity
			sts.Spec.Template.Spec.Tolerations = h.Spec.Scheduling.Tolerations
			sts.Spec.Template.Spec.NodeSelector = h.Spec.Scheduling.NodeSelector
		}
		sts.Spec.Template.Spec.TopologySpreadConstraints = appendHAModeTopologySpreadConstraints(h)

		if semver.Compare(fmt.Sprintf("v%s", h.Spec.Version), "v5.2.0") == 1 {
			sts.Spec.Template.Spec.Containers[0].ReadinessProbe.ProbeHandler.HTTPGet.Path = "/hazelcast/health/ready"
		}

		ic, err := initContainer(h, pvcName)
		if err != nil {
			return err
		}
		sts.Spec.Template.Spec.InitContainers = []v1.Container{ic}

		sts.Spec.Template.Spec.Volumes = volumes(h)
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = hzContainerVolumeMounts(h, pvcName)
		sts.Spec.Template.Spec.Containers[1].VolumeMounts = sidecarVolumeMounts(h, pvcName)
		if h.Spec.Agent.Resources != nil {
			sts.Spec.Template.Spec.Containers[1].Resources = *h.Spec.Agent.Resources
		}
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Statefulset", h.Name, "result", opResult)
	}
	return err
}

func persistentVolumeClaims(h *hazelcastv1alpha1.Hazelcast, pvcName string) []v1.PersistentVolumeClaim {
	var pvcs []v1.PersistentVolumeClaim
	if h.Spec.Persistence.IsEnabled() {
		pvcs = append(pvcs, v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        pvcName,
				Namespace:   h.Namespace,
				Labels:      labels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: h.Spec.Persistence.PVC.AccessModes,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						corev1.ResourceStorage: *h.Spec.Persistence.PVC.RequestStorage,
					},
				},
				StorageClassName: h.Spec.Persistence.PVC.StorageClassName,
			},
		})
	}
	if h.Spec.CPSubsystem.IsEnabled() && h.Spec.CPSubsystem.IsPVC() {
		pvcs = append(pvcs, v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        n.CPPersistenceVolumeName,
				Namespace:   h.Namespace,
				Labels:      labels(h),
				Annotations: h.Spec.Annotations,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: h.Spec.CPSubsystem.PVC.AccessModes,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						corev1.ResourceStorage: *h.Spec.CPSubsystem.PVC.RequestStorage,
					},
				},
				StorageClassName: h.Spec.CPSubsystem.PVC.StorageClassName,
			},
		})
	}
	return pvcs
}

func localDevicePersistentVolumeClaim(h *hazelcastv1alpha1.Hazelcast) []v1.PersistentVolumeClaim {
	var pvcs []v1.PersistentVolumeClaim
	for _, localDeviceConfig := range h.Spec.LocalDevices {
		pvcs = append(pvcs, v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      localDeviceConfig.Name,
				Namespace: h.Namespace,
				Labels:    labels(h),
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: localDeviceConfig.PVC.AccessModes,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						corev1.ResourceStorage: *localDeviceConfig.PVC.RequestStorage,
					},
				},
				StorageClassName: localDeviceConfig.PVC.StorageClassName,
			},
		})
	}
	return pvcs
}

func sidecarContainer(h *hazelcastv1alpha1.Hazelcast) v1.Container {
	return v1.Container{
		Name:  n.SidecarAgent,
		Image: h.AgentDockerImage(),
		Ports: []v1.ContainerPort{{
			ContainerPort: n.DefaultAgentPort,
			Name:          n.SidecarAgent,
			Protocol:      v1.ProtocolTCP,
		}},
		Args: []string{"sidecar"},
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
		SecurityContext: containerSecurityContext(),
	}
}

func hazelcastContainerWanRepPorts(h *hazelcastv1alpha1.Hazelcast) []v1.ContainerPort {
	// If WAN is not configured, use the default port for it
	if h.Spec.AdvancedNetwork == nil || len(h.Spec.AdvancedNetwork.WAN) == 0 {
		return []v1.ContainerPort{{
			ContainerPort: n.WanDefaultPort,
			Name:          n.WanDefaultPortName,
			Protocol:      v1.ProtocolTCP,
		}}
	}

	var c []v1.ContainerPort
	for _, w := range h.Spec.AdvancedNetwork.WAN {
		for i := 0; i < int(w.PortCount); i++ {
			c = append(c, v1.ContainerPort{
				ContainerPort: int32(int(w.Port) + i),
				Name:          fmt.Sprintf("%s%s-%s", n.WanPortNamePrefix, w.Name, strconv.Itoa(i)),
				Protocol:      v1.ProtocolTCP,
			})
		}
	}

	return c
}

func podSecurityContext() *v1.PodSecurityContext {
	// Openshift assigns user and fsgroup ids itself
	if platform.GetType() == platform.OpenShift {
		return &v1.PodSecurityContext{
			RunAsNonRoot: pointer.Bool(true),
		}
	}

	return &v1.PodSecurityContext{
		FSGroup:      pointer.Int64(65534),
		RunAsNonRoot: pointer.Bool(true),
		// Have to give userID otherwise Kubelet fails to create the pod
		// saying userID must be numberic, Hazelcast image's default userID is "hazelcast"
		// UBI images prohibits all numeric userIDs https://access.redhat.com/solutions/3103631
		RunAsUser: pointer.Int64(65534),
	}
}

func containerSecurityContext() *v1.SecurityContext {
	return &v1.SecurityContext{
		RunAsNonRoot:             pointer.Bool(true),
		Privileged:               pointer.Bool(false),
		ReadOnlyRootFilesystem:   pointer.Bool(true),
		AllowPrivilegeEscalation: pointer.Bool(false),
		Capabilities: &v1.Capabilities{
			Drop: []v1.Capability{"ALL"},
		},
	}
}

func initContainer(h *hazelcastv1alpha1.Hazelcast, pvcName string) (v1.Container, error) {
	c := corev1.Container{
		Name:  n.InitContainer,
		Image: h.AgentDockerImage(),
		Args:  []string{"execute-multiple-commands"},
		Env: []v1.EnvVar{
			{
				Name:  "CONFIG_FILE",
				Value: n.InitConfigDir + n.InitConfigFile,
			},
			{
				Name: "HOSTNAME",
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
		SecurityContext:          containerSecurityContext(),
		ImagePullPolicy:          corev1.PullIfNotPresent,
		VolumeMounts: []v1.VolumeMount{

			{
				Name:      n.InitConfigMap,
				MountPath: n.InitConfigDir,
			},
		},
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsBucketEnabled() ||
		h.Spec.DeprecatedUserCodeDeployment.IsRemoteURLsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, ucdBucketAgentVolumeMount())
	}

	if h.Spec.JetEngineConfiguration.IsBucketEnabled() ||
		h.Spec.JetEngineConfiguration.IsRemoteURLsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, jetJobJarsVolumeMount())
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, ucnBucketVolumeMount())
	}

	if h.Spec.Persistence.IsEnabled() {
		c.VolumeMounts = append(c.VolumeMounts, persistenceVolumeMount(pvcName))
	}

	return c, nil
}

func persistenceVolumeMount(pvcName string) v1.VolumeMount {
	return v1.VolumeMount{
		Name:      pvcName,
		MountPath: n.PersistenceMountPath,
	}
}

func ucnBucketVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      n.UCNVolumeName,
		MountPath: n.UCNBucketPath,
	}
}

func ucdBucketAgentVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      n.UserCodeBucketVolumeName,
		MountPath: n.UserCodeBucketPath,
	}
}

func jetJobJarsVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      n.JetJobJarsVolumeName,
		MountPath: n.JetJobJarsPath,
	}
}

func tmpDirVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      n.TmpDirVolName,
		MountPath: "/tmp",
	}
}

func ucdURLAgentVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      n.UserCodeURLVolumeName,
		MountPath: n.UserCodeURLPath,
	}
}

func volumes(h *hazelcastv1alpha1.Hazelcast) []v1.Volume {
	vols := []v1.Volume{
		{
			Name: n.HazelcastStorageName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName:  h.Name,
					DefaultMode: pointer.Int32(420),
				},
			},
		},
		{
			Name: n.InitConfigMap,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: h.Name + n.InitSuffix,
					},
					DefaultMode: pointer.Int32(420),
				},
			},
		},
		emptyDirVolume(n.UserCodeBucketVolumeName),
		emptyDirVolume(n.UserCodeURLVolumeName),
		emptyDirVolume(n.JetJobJarsVolumeName),
		emptyDirVolume(n.TmpDirVolName),
		tlsVolume(h),
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		vols = append(vols, emptyDirVolume(n.UCNVolumeName))
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsConfigMapEnabled() {
		vols = append(vols, configMapVolumes(ucdConfigMapName(h), h.Spec.DeprecatedUserCodeDeployment.RemoteFileConfiguration)...)
	}

	if h.Spec.JetEngineConfiguration.IsConfigMapEnabled() {
		vols = append(vols, configMapVolumes(jetConfigMapName, h.Spec.JetEngineConfiguration.RemoteFileConfiguration)...)
	}

	return vols
}

func emptyDirVolume(name string) v1.Volume {
	return v1.Volume{
		Name: name,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
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

type ConfigMapVolumeName func(cm string) string

func jetConfigMapName(cm string) string {
	return n.JetConfigMapNamePrefix + cm
}

func ucdConfigMapName(h *hazelcastv1alpha1.Hazelcast) ConfigMapVolumeName {
	return func(cm string) string {
		return n.UserCodeConfigMapNamePrefix + cm + h.Spec.DeprecatedUserCodeDeployment.TriggerSequence
	}
}

func configMapVolumes(nameFn ConfigMapVolumeName, rfc hazelcastv1alpha1.RemoteFileConfiguration) []corev1.Volume {
	var vols []corev1.Volume
	for _, cm := range rfc.ConfigMaps {
		vols = append(vols, corev1.Volume{
			Name: nameFn(cm),
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

func sidecarVolumeMounts(h *hazelcastv1alpha1.Hazelcast, pvcName string) []v1.VolumeMount {
	vm := []v1.VolumeMount{
		{
			Name:      n.MTLSCertSecretName,
			MountPath: n.MTLSCertPath,
		},
		jetJobJarsVolumeMount(),
		ucdBucketAgentVolumeMount(),
	}
	if h.Spec.UserCodeNamespaces.IsEnabled() {
		vm = append(vm, ucnBucketVolumeMount())
	}
	if h.Spec.Persistence.IsEnabled() {
		vm = append(vm, persistenceVolumeMount(pvcName))
	}
	return vm
}

func hzContainerVolumeMounts(h *hazelcastv1alpha1.Hazelcast, pvcName string) []v1.VolumeMount {
	mounts := []v1.VolumeMount{
		{
			Name:      n.HazelcastStorageName,
			MountPath: n.HazelcastMountPath,
		},
		ucdBucketAgentVolumeMount(),
		ucdURLAgentVolumeMount(),
		jetJobJarsVolumeMount(),
		// /tmp dir is overriden with emptyDir because Hazelcast fails to start with
		// read-only rootFileSystem when persistence is enabled because it tries to write
		// into /tmp dir.
		// /tmp dir is also needed for Jet Job submission and UCD from client/CLC.
		tmpDirVolumeMount(),
	}

	if h.Spec.Persistence.IsEnabled() {
		mounts = append(mounts, v1.VolumeMount{
			Name:      pvcName,
			MountPath: n.PersistenceMountPath,
		})
	}

	if h.Spec.UserCodeNamespaces.IsEnabled() {
		mounts = append(mounts, ucnBucketVolumeMount())
	}

	if h.Spec.CPSubsystem.IsEnabled() && h.Spec.CPSubsystem.IsPVC() {
		mounts = append(mounts, v1.VolumeMount{
			Name:      n.CPPersistenceVolumeName,
			MountPath: n.CPBaseDir,
		})
	}

	if h.Spec.IsTieredStorageEnabled() {
		mounts = append(mounts, localDeviceVolumeMounts(h)...)
	}

	if h.Spec.DeprecatedUserCodeDeployment.IsConfigMapEnabled() {
		mounts = append(mounts,
			configMapVolumeMounts(ucdConfigMapName(h), h.Spec.DeprecatedUserCodeDeployment.RemoteFileConfiguration, n.UserCodeConfigMapPath)...)
	}

	if h.Spec.JetEngineConfiguration.IsConfigMapEnabled() {
		mounts = append(mounts,
			configMapVolumeMounts(jetConfigMapName, h.Spec.JetEngineConfiguration.RemoteFileConfiguration, n.JetJobJarsPath)...)
	}
	return mounts
}

func localDeviceVolumeMounts(h *hazelcastv1alpha1.Hazelcast) []v1.VolumeMount {
	var vms []v1.VolumeMount
	for _, ld := range h.Spec.LocalDevices {
		vms = append(vms, v1.VolumeMount{
			Name:      ld.Name,
			MountPath: path.Join(n.TieredStorageBaseDir, ld.Name),
		})
	}
	return vms
}

func configMapVolumeMounts(nameFn ConfigMapVolumeName, rfc hazelcastv1alpha1.RemoteFileConfiguration, mountPath string) []corev1.VolumeMount {
	var vms []corev1.VolumeMount
	for _, cm := range rfc.ConfigMaps {
		vms = append(vms, corev1.VolumeMount{
			Name:      nameFn(cm),
			MountPath: path.Join(mountPath, cm),
		})
	}
	return vms
}

// persistenceStartupAction performs the action specified in the h.Spec.Persistence.StartupAction if
// the persistence is enabled and if the Hazelcast is not yet running
func (r *HazelcastReconciler) persistenceStartupAction(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	if !h.Spec.Persistence.IsEnabled() ||
		h.Spec.Persistence.StartupAction == "" ||
		h.Status.Phase == hazelcastv1alpha1.Running {
		return nil
	}
	logger.Info("Persistence enabled with startup action.", "action", h.Spec.Persistence.StartupAction)
	if h.Spec.Persistence.StartupAction == hazelcastv1alpha1.ForceStart {
		return NewRestClient(h).ForceStart(ctx)
	}
	if h.Spec.Persistence.StartupAction == hazelcastv1alpha1.PartialStart {
		return NewRestClient(h).PartialStart(ctx)
	}
	return nil
}

func (r *HazelcastReconciler) ensureClusterActive(ctx context.Context, client hzclient.Client, h *hazelcastv1alpha1.Hazelcast) error {
	// make sure restore is active
	if !h.Spec.Persistence.IsRestoreEnabled() {
		return nil
	}

	// make sure restore was successful
	if h.Status.Restore == (hazelcastv1alpha1.RestoreStatus{}) {
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

var illegalClusterType = errs.New("only enterprise clusters are supported")

func (r *HazelcastReconciler) isEnterpriseCluster(ctx context.Context, client hzclient.Client, h *hazelcastv1alpha1.Hazelcast) (bool, error) {
	req := codec.EncodeClientAuthenticationRequest(
		h.Name,
		"",
		"",
		clientTypes.NewUUID(),
		"GOO",
		1,
		"1.5.0",
		"operator",
		[]string{})
	resp, err := client.InvokeOnRandomTarget(ctx, req, nil)
	if err != nil {
		return false, err
	}
	_, _, _, _, _, _, _, failOverSupported := codec.DecodeClientAuthenticationResponse(resp)
	return failOverSupported, nil
}

func appendHAModeTopologySpreadConstraints(h *hazelcastv1alpha1.Hazelcast) []v1.TopologySpreadConstraint {
	var topologySpreadConstraints []v1.TopologySpreadConstraint
	if h.Spec.Scheduling != nil {
		topologySpreadConstraints = append(topologySpreadConstraints, h.Spec.Scheduling.TopologySpreadConstraints...)
	}
	if h.Spec.HighAvailabilityMode != "" {
		switch h.Spec.HighAvailabilityMode {
		case "NODE":
			topologySpreadConstraints = append(topologySpreadConstraints,
				v1.TopologySpreadConstraint{
					MaxSkew:           1,
					TopologyKey:       "kubernetes.io/hostname",
					WhenUnsatisfiable: v1.ScheduleAnyway,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: util.Labels(h)},
				})
		case "ZONE":
			topologySpreadConstraints = append(topologySpreadConstraints,
				v1.TopologySpreadConstraint{
					MaxSkew:           1,
					TopologyKey:       "topology.kubernetes.io/zone",
					WhenUnsatisfiable: v1.ScheduleAnyway,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: util.Labels(h)},
				})
		}
	}
	return topologySpreadConstraints
}

func env(h *hazelcastv1alpha1.Hazelcast) []v1.EnvVar {
	envs := []v1.EnvVar{
		{
			Name:  JavaOpts,
			Value: javaOPTS(h),
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
			Value: `{"time":"%date{ISO8601}", "level": "%level", "threadName": "%threadName", "logger": "%logger{36}", "msg": "%enc{%m %xEx}{JSON}"}%n`,
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
	if h.Spec.GetLicenseKeySecretName() != "" {
		envs = append(envs,
			v1.EnvVar{
				Name: hzLicenseKey,
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: h.Spec.GetLicenseKeySecretName(),
						},
						Key: n.LicenseDataKey,
					},
				},
			})
	}

	envs = append(envs, h.Spec.Env...)

	return envs
}

func javaOPTS(h *hazelcastv1alpha1.Hazelcast) string {
	b := strings.Builder{}
	b.WriteString("-Dhazelcast.config=" + path.Join(n.HazelcastMountPath, "hazelcast.yaml"))

	// we should configure JVM to respect container’s resource limits
	b.WriteString(" -XX:+UseContainerSupport")

	jvmMemory := h.Spec.JVM.GetMemory()

	// in addition, we allow user to set explicit memory limits
	if v := jvmMemory.GetInitialRAMPercentage(); v != "" {
		b.WriteString(" -XX:InitialRAMPercentage=" + v)
	}

	if v := jvmMemory.GetMinRAMPercentage(); v != "" {
		b.WriteString(" -XX:MinRAMPercentage=" + v)
	}

	if v := jvmMemory.GetMaxRAMPercentage(); v != "" {
		b.WriteString(" -XX:MaxRAMPercentage=" + v)
	}

	args := h.Spec.JVM.GetArgs()
	if len(args) != 0 {
		args = mergeJVMArgs(args)
	} else {
		for k, v := range DefaultJavaOptions {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}
	}

	for _, a := range args {
		b.WriteString(fmt.Sprintf(" %s", a))
	}

	jvmGC := h.Spec.JVM.GCConfig()

	if jvmGC.IsLoggingEnabled() {
		b.WriteString(" -verbose:gc")
	}

	if v := jvmGC.GetCollector(); v != "" {
		switch v {
		case hazelcastv1alpha1.GCTypeSerial:
			b.WriteString(" -XX:+UseSerialGC")
		case hazelcastv1alpha1.GCTypeParallel:
			b.WriteString(" -XX:+UseParallelGC")
		case hazelcastv1alpha1.GCTypeG1:
			b.WriteString(" -XX:+UseG1GC")
		}
	}

	return b.String()
}

func mergeJVMArgs(args []string) []string {
	configuredArgs := make(map[string]struct{})
	for _, a := range args {
		jvmOptionKeyValue := strings.Split(a, "=")
		configuredArgs[strings.TrimSpace(jvmOptionKeyValue[0])] = struct{}{}
	}

	for k, v := range DefaultJavaOptions {
		_, ok := configuredArgs[k]
		if !ok {
			args = append(args, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return args
}

func javaClassPath(h *hazelcastv1alpha1.Hazelcast) string {
	b := []string{
		path.Join(n.UserCodeBucketPath, "*"),
		path.Join(n.UserCodeURLPath, "*")}

	if h.Spec.DeprecatedUserCodeDeployment != nil {
		for _, cm := range h.Spec.DeprecatedUserCodeDeployment.RemoteFileConfiguration.ConfigMaps {
			b = append(b, path.Join(n.UserCodeConfigMapPath, cm, "*"))
		}
	}

	return strings.Join(b, ":")
}

func statefulSetAnnotations(sts *appsv1.StatefulSet, h *hazelcastv1alpha1.Hazelcast) map[string]string {
	annotations := make(map[string]string)

	// copy old annotations
	for name, value := range sts.Annotations {
		annotations[name] = value
	}

	// copy user annotations
	for name, value := range h.Spec.Annotations {
		annotations[name] = value
	}

	if !h.Spec.ExposeExternally.IsSmart() {
		return annotations
	}

	// overwrite
	annotations[n.ServicePerPodCountAnnotation] = strconv.Itoa(int(*h.Spec.ClusterSize))

	return annotations
}

func podAnnotations(annotations map[string]string, h *hazelcastv1alpha1.Hazelcast) (map[string]string, error) {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if h.Spec.ExposeExternally.IsSmart() {
		annotations[n.ExposeExternallyAnnotation] = string(h.Spec.ExposeExternally.MemberAccessType())
	} else {
		delete(annotations, n.ExposeExternallyAnnotation)
	}

	cfg := config.HazelcastWrapper{Hazelcast: configForcingRestart(hazelcastBasicConfig(h))}
	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	annotations[n.CurrentHazelcastConfigForcingRestartChecksum] = fmt.Sprint(crc32.ChecksumIEEE(cfgYaml))

	return annotations, nil
}

func hazelcastContainerPorts(h *hazelcastv1alpha1.Hazelcast) []v1.ContainerPort {
	ports := []v1.ContainerPort{{
		ContainerPort: n.DefaultHzPort,
		Name:          n.Hazelcast,
		Protocol:      v1.ProtocolTCP,
	}, {
		ContainerPort: n.MemberServerSocketPort,
		Name:          n.MemberPortName,
		Protocol:      v1.ProtocolTCP,
	}, {
		ContainerPort: n.RestServerSocketPort,
		Name:          n.RestPortName,
		Protocol:      v1.ProtocolTCP,
	},
	}

	ports = append(ports, hazelcastContainerWanRepPorts(h)...)
	return ports
}

func configForcingRestart(hz config.Hazelcast) config.Hazelcast {
	// Apart from these changes, any change in the statefulset spec, labels, annotation can force a restart.
	return config.Hazelcast{
		ClusterName:        hz.ClusterName,
		Jet:                hz.Jet,
		UserCodeDeployment: hz.UserCodeDeployment,
		Properties:         hz.Properties,
		AdvancedNetwork: config.AdvancedNetwork{
			ClientServerSocketEndpointConfig: config.ClientServerSocketEndpointConfig{
				SSL: hz.AdvancedNetwork.ClientServerSocketEndpointConfig.SSL,
			},
			MemberServerSocketEndpointConfig: config.MemberServerSocketEndpointConfig{
				SSL: hz.AdvancedNetwork.MemberServerSocketEndpointConfig.SSL,
			},
		},
		CPSubsystem: hz.CPSubsystem,
	}
}

func metadata(h *hazelcastv1alpha1.Hazelcast) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        h.Name,
		Namespace:   h.Namespace,
		Labels:      labels(h),
		Annotations: h.Spec.Annotations,
	}
}

func labels(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	l := make(map[string]string)

	// copy user labels
	for name, value := range h.Spec.Labels {
		l[name] = value
	}

	// make sure we overwrite user labels
	l[n.ApplicationNameLabel] = n.Hazelcast
	l[n.ApplicationInstanceNameLabel] = h.GetName()
	l[n.ApplicationManagedByLabel] = n.OperatorName

	return l
}

func serviceAccountName(h *hazelcastv1alpha1.Hazelcast) string {
	if h.Spec.ServiceAccountName != "" {
		return h.Spec.ServiceAccountName
	}
	return h.Name
}

func (r *HazelcastReconciler) updateLastSuccessfulConfiguration(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	opResult, err := util.Update(ctx, r.Client, h, func() error {
		controller.InsertLastSuccessfullyAppliedSpec(h.Spec, h)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Hazelcast Annotation", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) updateLastAppliedSpec(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, logger logr.Logger) error {
	opResult, err := util.Update(ctx, r.Client, h, func() error {
		controller.InsertLastAppliedSpec(h.Spec, h)
		return nil
	})
	if opResult != controllerutil.OperationResultNone {
		logger.Info("Operation result", "Hazelcast Annotation", h.Name, "result", opResult)
	}
	return err
}

func (r *HazelcastReconciler) unmarshalHazelcastSpec(h *hazelcastv1alpha1.Hazelcast, rawLastSpec string) (*hazelcastv1alpha1.HazelcastSpec, error) {
	hs, err := json.Marshal(h.Spec)
	if err != nil {
		err = fmt.Errorf("error marshaling Hazelcast as JSON: %w", err)
		return nil, err
	}
	if rawLastSpec == string(hs) {
		return &h.Spec, nil
	}
	lastSpec := &hazelcastv1alpha1.HazelcastSpec{}
	err = json.Unmarshal([]byte(rawLastSpec), lastSpec)
	if err != nil {
		err = fmt.Errorf("error unmarshaling Last HZ Spec: %w", err)
		return nil, err
	}
	return lastSpec, nil
}

func (r *HazelcastReconciler) detectNewExecutorServices(h *hazelcastv1alpha1.Hazelcast, lastSpec *hazelcastv1alpha1.HazelcastSpec) (map[string]interface{}, error) {
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
	esInput.UserCodeNamespace = es.UserCodeNamespace
}

func fillAddDurableExecutorServiceInput(esInput *codecTypes.DurableExecutorServiceConfig, es hazelcastv1alpha1.DurableExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.Capacity = es.Capacity
	esInput.Durability = es.Durability
	esInput.UserCodeNamespace = es.UserCodeNamespace
}

func fillAddScheduledExecutorServiceInput(esInput *codecTypes.ScheduledExecutorServiceConfig, es hazelcastv1alpha1.ScheduledExecutorServiceConfiguration) {
	esInput.Name = es.Name
	esInput.PoolSize = es.PoolSize
	esInput.Capacity = es.Capacity
	esInput.CapacityPolicy = es.CapacityPolicy
	esInput.Durability = es.Durability
	esInput.UserCodeNamespace = es.UserCodeNamespace
}
