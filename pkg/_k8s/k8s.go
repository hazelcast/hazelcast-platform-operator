package k8s

import (
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/gruntwork-io/terratest/modules/testing"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
)

// JobNotSucceeded is returned when a Kubernetes job is not Succeeded
type JobNotSucceeded = k8s.JobNotSucceeded

// KubectlOptions represents common options necessary to specify for all Kubectl calls
type KubectlOptions = k8s.KubectlOptions

// KubeResourceType is an enum representing known resource types that can support port forwarding
type KubeResourceType = k8s.KubeResourceType

// MalformedNodeID is returned when a Kubernetes node has a malformed node id scheme
type MalformedNodeID = k8s.MalformedNodeID

// NodeHasNoHostname is returned when a Kubernetes node has no discernible hostname
type NodeHasNoHostname = k8s.NodeHasNoHostname

// NoNodesInKubernetes is returned when the Kubernetes cluster has no nodes registered.
type NoNodesInKubernetes = k8s.NoNodesInKubernetes

// PodNotAvailable is returned when a Kubernetes service is not yet available to accept traffic.
type PodNotAvailable = k8s.PodNotAvailable

// ServiceNotAvailable is returned when a Kubernetes service is not yet available to accept traffic.
type ServiceNotAvailable = k8s.ServiceNotAvailable

// UnknownServicePort is returned when the given service port is not an exported port of the service.
type UnknownServicePort = k8s.UnknownServicePort

// UnknownServiceType is returned when a Kubernetes service has a type that is not yet handled by the test functions.
type UnknownServiceType = k8s.UnknownServiceType

// Tunnel is the main struct that configures and manages port forwarding tunnels to Kubernetes resources.
type Tunnel = k8s.Tunnel

// GetNodes queries Kubernetes for information about the worker nodes registered to the cluster. If anything goes wrong,
// the function will automatically fail the test.
func GetNodes(t testing.TestingT, options *KubectlOptions) []corev1.Node {
	return k8s.GetNodes(t, options)
}

// GetNodesE queries Kubernetes for information about the worker nodes registered to the cluster.
func GetNodesE(t testing.TestingT, options *KubectlOptions) ([]corev1.Node, error) {
	return k8s.GetNodesE(t, options)
}

// GetNodesByFilterE queries Kubernetes for information about the worker nodes registered to the cluster, filtering the
// list of nodes using the provided ListOptions.
func GetNodesByFilterE(t testing.TestingT, options *KubectlOptions, filter metav1.ListOptions) ([]corev1.Node, error) {
	return k8s.GetNodesByFilterE(t, options, filter)
}

// GetReadyNodes queries Kubernetes for information about the worker nodes registered to the cluster and only returns
// those that are in the ready state. If anything goes wrong, the function will automatically fail the test.
func GetReadyNodes(t testing.TestingT, options *KubectlOptions) []corev1.Node {
	return k8s.GetReadyNodes(t, options)
}

// GetReadyNodesE queries Kubernetes for information about the worker nodes registered to the cluster and only returns
// those that are in the ready state.
func GetReadyNodesE(t testing.TestingT, options *KubectlOptions) ([]corev1.Node, error) {
	return k8s.GetReadyNodesE(t, options)
}

// IsNodeReady takes a Kubernetes Node information object and checks if the Node is in the ready state.
func IsNodeReady(node corev1.Node) bool { return k8s.IsNodeReady(node) }

// WaitUntilAllNodesReady continuously polls the Kubernetes cluster until all nodes in the cluster reach the ready
// state, or runs out of retries. Will fail the test immediately if it times out.
func WaitUntilAllNodesReady(t testing.TestingT, options *KubectlOptions, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilAllNodesReady(t, options, retries, sleepBetweenRetries)
}

// WaitUntilAllNodesReadyE continuously polls the Kubernetes cluster until all nodes in the cluster reach the ready
// state, or runs out of retries.
func WaitUntilAllNodesReadyE(t testing.TestingT, options *KubectlOptions, retries int, sleepBetweenRetries time.Duration) error {
	return k8s.WaitUntilAllNodesReadyE(t, options, retries, sleepBetweenRetries)
}

// AreAllNodesReady checks if all nodes are ready in the Kubernetes cluster targeted by the current config context
func AreAllNodesReady(t testing.TestingT, options *KubectlOptions) bool {
	return k8s.AreAllNodesReady(t, options)
}

// AreAllNodesReadyE checks if all nodes are ready in the Kubernetes cluster targeted by the current config context. If
// false, returns an error indicating the reason.
func AreAllNodesReadyE(t testing.TestingT, options *KubectlOptions) (bool, error) {
	return k8s.AreAllNodesReadyE(t, options)
}

// GetSecret returns a Kubernetes secret resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions. This will fail the test if there is an error.
func GetSecret(t testing.TestingT, options *KubectlOptions, secretName string) *corev1.Secret {
	return k8s.GetSecret(t, options, secretName)
}

// GetSecretE returns a Kubernetes secret resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions.
func GetSecretE(t testing.TestingT, options *KubectlOptions, secretName string) (*corev1.Secret, error) {
	return k8s.GetSecretE(t, options, secretName)
}

// WaitUntilSecretAvailable waits until the secret is present on the cluster in cases where it is not immediately
// available (for example, when using ClusterIssuer to request a certificate).
func WaitUntilSecretAvailable(t testing.TestingT, options *KubectlOptions, secretName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilSecretAvailable(t, options, secretName, retries, sleepBetweenRetries)
}

// GetKubernetesClientE returns a Kubernetes API client that can be used to make requests.
func GetKubernetesClientE(t testing.TestingT) (*kubernetes.Clientset, error) {
	return k8s.GetKubernetesClientE(t)
}

// GetKubernetesClientFromOptionsE returns a Kubernetes API client given a configured KubectlOptions object.
func GetKubernetesClientFromOptionsE(t testing.TestingT, options *KubectlOptions) (*kubernetes.Clientset, error) {
	return k8s.GetKubernetesClientFromOptionsE(t, options)
}

// ListDaemonSets will look for daemonsets in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListDaemonSets(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []appsv1.DaemonSet {
	return k8s.ListDaemonSets(t, options, filters)
}

// ListDaemonSetsE will look for daemonsets in the given namespace that match the given filters and return them.
func ListDaemonSetsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]appsv1.DaemonSet, error) {
	return k8s.ListDaemonSetsE(t, options, filters)
}

// GetDaemonSet returns a Kubernetes daemonset resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetDaemonSet(t testing.TestingT, options *KubectlOptions, daemonSetName string) *appsv1.DaemonSet {
	return k8s.GetDaemonSet(t, options, daemonSetName)
}

// GetDaemonSetE returns a Kubernetes daemonset resource in the provided namespace with the given name.
func GetDaemonSetE(t testing.TestingT, options *KubectlOptions, daemonSetName string) (*appsv1.DaemonSet, error) {
	return k8s.GetDaemonSetE(t, options, daemonSetName)
}

// NewPodNotAvailableError returnes a PodNotAvailable struct when Kubernetes deems a pod is not available
func NewPodNotAvailableError(pod *corev1.Pod) PodNotAvailable {
	return k8s.NewPodNotAvailableError(pod)
}

// NewJobNotSucceeded returnes a JobNotSucceeded when the status of the job is not Succeeded
func NewJobNotSucceeded(job *batchv1.Job) JobNotSucceeded { return k8s.NewJobNotSucceeded(job) }

// NewServiceNotAvailableError returnes a ServiceNotAvailable struct when Kubernetes deems a service is not available
func NewServiceNotAvailableError(service *corev1.Service) ServiceNotAvailable {
	return k8s.NewServiceNotAvailableError(service)
}

// NewUnknownServiceTypeError returns an UnknownServiceType struct when is it deemed that Kubernetes does not know the service type provided
func NewUnknownServiceTypeError(service *corev1.Service) UnknownServiceType {
	return k8s.NewUnknownServiceTypeError(service)
}

// NewUnknownServicePortError returns an UnknownServicePort struct when it is deemed that Kuberenetes does not know of the provided Service Port
func NewUnknownServicePortError(service *corev1.Service, port int32) UnknownServicePort {
	return k8s.NewUnknownServicePortError(service, port)
}

// NewNoNodesInKubernetesError returns a NoNodesInKubernetes struct when it is deemed that there are no Kubernetes nodes registered
func NewNoNodesInKubernetesError() NoNodesInKubernetes { return k8s.NewNoNodesInKubernetesError() }

// NewNodeHasNoHostnameError returns a NodeHasNoHostname struct when it is deemed that the provided node has no hostname
func NewNodeHasNoHostnameError(node *corev1.Node) NodeHasNoHostname {
	return k8s.NewNodeHasNoHostnameError(node)
}

// NewMalformedNodeIDError returns a MalformedNodeID struct when Kubernetes deems that a NodeID is malformed
func NewMalformedNodeIDError(node *corev1.Node) MalformedNodeID {
	return k8s.NewMalformedNodeIDError(node)
}

// CreateNamespace will create a new Kubernetes namespace on the cluster targeted by the provided options. This will
// fail the test if there is an error in creating the namespace.
func CreateNamespace(t testing.TestingT, options *KubectlOptions, namespaceName string) {
	k8s.CreateNamespace(t, options, namespaceName)
}

// CreateNamespaceE will create a new Kubernetes namespace on the cluster targeted by the provided options.
func CreateNamespaceE(t testing.TestingT, options *KubectlOptions, namespaceName string) error {
	return k8s.CreateNamespaceE(t, options, namespaceName)
}

// CreateNamespaceWithMetadataE will create a new Kubernetes namespace on the cluster targeted by the provided options and
// with the provided metadata. This method expects the entire namespace ObjectMeta to be passed in, so you'll need to set the name within the ObjectMeta struct yourself.
func CreateNamespaceWithMetadataE(t testing.TestingT, options *KubectlOptions, namespaceObjectMeta metav1.ObjectMeta) error {
	return k8s.CreateNamespaceWithMetadataE(t, options, namespaceObjectMeta)
}

// CreateNamespaceWithMetadata will create a new Kubernetes namespace on the cluster targeted by the provided options and
// with the provided metadata. This method expects the entire namespace ObjectMeta to be passed in, so you'll need to set the name within the ObjectMeta struct yourself.
// This will fail the test if there is an error while creating the namespace.
func CreateNamespaceWithMetadata(t testing.TestingT, options *KubectlOptions, namespaceObjectMeta metav1.ObjectMeta) {
	k8s.CreateNamespaceWithMetadata(t, options, namespaceObjectMeta)
}

// GetNamespace will query the Kubernetes cluster targeted by the provided options for the requested namespace. This will
// fail the test if there is an error in getting the namespace or if the namespace doesn't exist.
func GetNamespace(t testing.TestingT, options *KubectlOptions, namespaceName string) *corev1.Namespace {
	return k8s.GetNamespace(t, options, namespaceName)
}

// GetNamespaceE will query the Kubernetes cluster targeted by the provided options for the requested namespace.
func GetNamespaceE(t testing.TestingT, options *KubectlOptions, namespaceName string) (*corev1.Namespace, error) {
	return k8s.GetNamespaceE(t, options, namespaceName)
}

// DeleteNamespace will delete the requested namespace from the Kubernetes cluster targeted by the provided options. This will
// fail the test if there is an error in creating the namespace.
func DeleteNamespace(t testing.TestingT, options *KubectlOptions, namespaceName string) {
	k8s.DeleteNamespace(t, options, namespaceName)
}

// DeleteNamespaceE will delete the requested namespace from the Kubernetes cluster targeted by the provided options.
func DeleteNamespaceE(t testing.TestingT, options *KubectlOptions, namespaceName string) error {
	return k8s.DeleteNamespaceE(t, options, namespaceName)
}

// ListIngresses will look for Ingress resources in the given namespace that match the given filters and return them.
// This will fail the test if there is an error.
func ListIngresses(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []networkingv1.Ingress {
	return k8s.ListIngresses(t, options, filters)
}

// ListIngressesE will look for Ingress resources in the given namespace that match the given filters and return them.
func ListIngressesE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]networkingv1.Ingress, error) {
	return k8s.ListIngressesE(t, options, filters)
}

// GetIngress returns a Kubernetes Ingress resource in the provided namespace with the given name. This will fail the
// test if there is an error.
func GetIngress(t testing.TestingT, options *KubectlOptions, ingressName string) *networkingv1.Ingress {
	return k8s.GetIngress(t, options, ingressName)
}

// GetIngressE returns a Kubernetes Ingress resource in the provided namespace with the given name.
func GetIngressE(t testing.TestingT, options *KubectlOptions, ingressName string) (*networkingv1.Ingress, error) {
	return k8s.GetIngressE(t, options, ingressName)
}

// IsIngressAvailable returns true if the Ingress endpoint is provisioned and available.
func IsIngressAvailable(ingress *networkingv1.Ingress) bool { return k8s.IsIngressAvailable(ingress) }

// WaitUntilIngressAvailable waits until the Ingress resource has an endpoint provisioned for it.
func WaitUntilIngressAvailable(t testing.TestingT, options *KubectlOptions, ingressName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilIngressAvailable(t, options, ingressName, retries, sleepBetweenRetries)
}

// ListIngressesV1Beta1 will look for Ingress resources in the given namespace that match the given filters and return
// them, using networking.k8s.io/v1beta1 API. This will fail the test if there is an error.
func ListIngressesV1Beta1(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []networkingv1beta1.Ingress {
	return k8s.ListIngressesV1Beta1(t, options, filters)
}

// ListIngressesV1Beta1E will look for Ingress resources in the given namespace that match the given filters and return
// them, using networking.k8s.io/v1beta1 API.
func ListIngressesV1Beta1E(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]networkingv1beta1.Ingress, error) {
	return k8s.ListIngressesV1Beta1E(t, options, filters)
}

// GetIngressV1Beta1 returns a Kubernetes Ingress resource in the provided namespace with the given name, using
// networking.k8s.io/v1beta1 API. This will fail the test if there is an error.
func GetIngressV1Beta1(t testing.TestingT, options *KubectlOptions, ingressName string) *networkingv1beta1.Ingress {
	return k8s.GetIngressV1Beta1(t, options, ingressName)
}

// GetIngressV1Beta1E returns a Kubernetes Ingress resource in the provided namespace with the given name, using
// networking.k8s.io/v1beta1.
func GetIngressV1Beta1E(t testing.TestingT, options *KubectlOptions, ingressName string) (*networkingv1beta1.Ingress, error) {
	return k8s.GetIngressV1Beta1E(t, options, ingressName)
}

// IsIngressAvailableV1Beta1 returns true if the Ingress endpoint is provisioned and available, using
// networking.k8s.io/v1beta1 API.
func IsIngressAvailableV1Beta1(ingress *networkingv1beta1.Ingress) bool {
	return k8s.IsIngressAvailableV1Beta1(ingress)
}

// WaitUntilIngressAvailableV1Beta1 waits until the Ingress resource has an endpoint provisioned for it, using
// networking.k8s.io/v1beta1 API.
func WaitUntilIngressAvailableV1Beta1(t testing.TestingT, options *KubectlOptions, ingressName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilIngressAvailableV1Beta1(t, options, ingressName, retries, sleepBetweenRetries)
}

// RunKubectl will call kubectl using the provided options and args, failing the test on error.
func RunKubectl(t testing.TestingT, options *KubectlOptions, args ...string) {
	k8s.RunKubectl(t, options, args...)
}

// RunKubectlE will call kubectl using the provided options and args.
func RunKubectlE(t testing.TestingT, options *KubectlOptions, args ...string) error {
	return k8s.RunKubectlE(t, options, args...)
}

// RunKubectlAndGetOutputE will call kubectl using the provided options and args, returning the output of stdout and
// stderr.
func RunKubectlAndGetOutputE(t testing.TestingT, options *KubectlOptions, args ...string) (string, error) {
	return k8s.RunKubectlAndGetOutputE(t, options, args...)
}

// KubectlDelete will take in a file path and delete it from the cluster targeted by KubectlOptions. If there are any
// errors, fail the test immediately.
func KubectlDelete(t testing.TestingT, options *KubectlOptions, configPath string) {
	k8s.KubectlDelete(t, options, configPath)
}

// KubectlDeleteE will take in a file path and delete it from the cluster targeted by KubectlOptions.
func KubectlDeleteE(t testing.TestingT, options *KubectlOptions, configPath string) error {
	return k8s.KubectlDeleteE(t, options, configPath)
}

// KubectlDeleteFromKustomize will take in a kustomization directory path and delete it from the cluster targeted by KubectlOptions. If there are any
// errors, fail the test immediately.
func KubectlDeleteFromKustomize(t testing.TestingT, options *KubectlOptions, configPath string) {
	k8s.KubectlDeleteFromKustomize(t, options, configPath)
}

// KubectlDeleteFromKustomizeE will take in a kustomization directory path and delete it from the cluster targeted by KubectlOptions.
func KubectlDeleteFromKustomizeE(t testing.TestingT, options *KubectlOptions, configPath string) error {
	return k8s.KubectlDeleteFromKustomizeE(t, options, configPath)
}

// KubectlDeleteFromString will take in a kubernetes resource config as a string and delete it on the cluster specified
// by the provided kubectl options.
func KubectlDeleteFromString(t testing.TestingT, options *KubectlOptions, configData string) {
	k8s.KubectlDeleteFromString(t, options, configData)
}

// KubectlDeleteFromStringE will take in a kubernetes resource config as a string and delete it on the cluster specified
// by the provided kubectl options. If it fails, this will return the error.
func KubectlDeleteFromStringE(t testing.TestingT, options *KubectlOptions, configData string) error {
	return k8s.KubectlDeleteFromStringE(t, options, configData)
}

// KubectlApply will take in a file path and apply it to the cluster targeted by KubectlOptions. If there are any
// errors, fail the test immediately.
func KubectlApply(t testing.TestingT, options *KubectlOptions, configPath string) {
	k8s.KubectlApply(t, options, configPath)
}

// KubectlApplyE will take in a file path and apply it to the cluster targeted by KubectlOptions.
func KubectlApplyE(t testing.TestingT, options *KubectlOptions, configPath string) error {
	return k8s.KubectlApplyE(t, options, configPath)
}

// KubectlApplyFromKustomize will take in a kustomization directory path and apply it to the cluster targeted by KubectlOptions. If there are any
// errors, fail the test immediately.
func KubectlApplyFromKustomize(t testing.TestingT, options *KubectlOptions, configPath string) {
	k8s.KubectlApplyFromKustomize(t, options, configPath)
}

// KubectlApplyFromKustomizeE will take in a kustomization directory path and delete it from the cluster targeted by KubectlOptions.
func KubectlApplyFromKustomizeE(t testing.TestingT, options *KubectlOptions, configPath string) error {
	return k8s.KubectlApplyFromKustomizeE(t, options, configPath)
}

// KubectlApplyFromString will take in a kubernetes resource config as a string and apply it on the cluster specified
// by the provided kubectl options.
func KubectlApplyFromString(t testing.TestingT, options *KubectlOptions, configData string) {
	k8s.KubectlApplyFromString(t, options, configData)
}

// KubectlApplyFromStringE will take in a kubernetes resource config as a string and apply it on the cluster specified
// by the provided kubectl options. If it fails, this will return the error.
func KubectlApplyFromStringE(t testing.TestingT, options *KubectlOptions, configData string) error {
	return k8s.KubectlApplyFromStringE(t, options, configData)
}

// StoreConfigToTempFile will store the provided config data to a temporary file created on the os and return the
// filename.
func StoreConfigToTempFile(t testing.TestingT, configData string) string {
	return k8s.StoreConfigToTempFile(t, configData)
}

// StoreConfigToTempFileE will store the provided config data to a temporary file created on the os and return the
// filename, or error.
func StoreConfigToTempFileE(t testing.TestingT, configData string) (string, error) {
	return k8s.StoreConfigToTempFileE(t, configData)
}

// GetRole returns a Kubernetes role resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions. This will fail the test if there is an error.
func GetRole(t testing.TestingT, options *KubectlOptions, roleName string) *rbacv1.Role {
	return k8s.GetRole(t, options, roleName)
}

// GetRoleE returns a Kubernetes role resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions.
func GetRoleE(t testing.TestingT, options *KubectlOptions, roleName string) (*rbacv1.Role, error) {
	return k8s.GetRoleE(t, options, roleName)
}

// NewTunnel creates a new tunnel with NewTunnelWithLogger, setting logger.Terratest as the logger.
func NewTunnel(kubectlOptions *KubectlOptions, resourceType KubeResourceType, resourceName string, local int, remote int) *Tunnel {
	return k8s.NewTunnel(kubectlOptions, resourceType, resourceName, local, remote)
}

// NewTunnelWithLogger will create a new Tunnel struct with the provided logger.
// Note that if you use 0 for the local port, an open port on the host system
// will be selected automatically, and the Tunnel struct will be updated with the selected port.
func NewTunnelWithLogger(kubectlOptions *KubectlOptions, resourceType KubeResourceType, resourceName string, local int, remote int, logger logger.TestLogger) *Tunnel {
	return k8s.NewTunnelWithLogger(kubectlOptions, resourceType, resourceName, local, remote, logger)
}

// GetAvailablePort retrieves an available port on the host machine. This delegates the port selection to the golang net
// library by starting a server and then checking the port that the server is using. This will fail the test if it could
// not find an available port.
func GetAvailablePort(t testing.TestingT) int { return k8s.GetAvailablePort(t) }

// GetAvailablePortE retrieves an available port on the host machine. This delegates the port selection to the golang net
// library by starting a server and then checking the port that the server is using.
func GetAvailablePortE(t testing.TestingT) (int, error) { return k8s.GetAvailablePortE(t) }

// GetNetworkPolicy returns a Kubernetes networkpolicy resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions. This will fail the test if there is an error.
func GetNetworkPolicy(t testing.TestingT, options *KubectlOptions, networkPolicyName string) *networkingv1.NetworkPolicy {
	return k8s.GetNetworkPolicy(t, options, networkPolicyName)
}

// GetNetworkPolicyE returns a Kubernetes networkpolicy resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions.
func GetNetworkPolicyE(t testing.TestingT, options *KubectlOptions, networkPolicyName string) (*networkingv1.NetworkPolicy, error) {
	return k8s.GetNetworkPolicyE(t, options, networkPolicyName)
}

// WaitUntilNetworkPolicyAvailable waits until the networkpolicy is present on the cluster in cases where it is not immediately
// available (for example, when using ClusterIssuer to request a certificate).
func WaitUntilNetworkPolicyAvailable(t testing.TestingT, options *KubectlOptions, networkPolicyName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilNetworkPolicyAvailable(t, options, networkPolicyName, retries, sleepBetweenRetries)
}

// ListServices will look for services in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListServices(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []corev1.Service {
	return k8s.ListServices(t, options, filters)
}

// ListServicesE will look for services in the given namespace that match the given filters and return them.
func ListServicesE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]corev1.Service, error) {
	return k8s.ListServicesE(t, options, filters)
}

// GetService returns a Kubernetes service resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetService(t testing.TestingT, options *KubectlOptions, serviceName string) *corev1.Service {
	return k8s.GetService(t, options, serviceName)
}

// GetServiceE returns a Kubernetes service resource in the provided namespace with the given name.
func GetServiceE(t testing.TestingT, options *KubectlOptions, serviceName string) (*corev1.Service, error) {
	return k8s.GetServiceE(t, options, serviceName)
}

// WaitUntilServiceAvailable waits until the service endpoint is ready to accept traffic.
func WaitUntilServiceAvailable(t testing.TestingT, options *KubectlOptions, serviceName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilServiceAvailable(t, options, serviceName, retries, sleepBetweenRetries)
}

// IsServiceAvailable returns true if the service endpoint is ready to accept traffic. Note that for Minikube, this
// function is moot as all services, even LoadBalancer, is available immediately.
func IsServiceAvailable(service *corev1.Service) bool { return k8s.IsServiceAvailable(service) }

// GetServiceEndpoint will return the service access point. If the service endpoint is not ready, will fail the test
// immediately.
func GetServiceEndpoint(t testing.TestingT, options *KubectlOptions, service *corev1.Service, servicePort int) string {
	return k8s.GetServiceEndpoint(t, options, service, servicePort)
}

// GetServiceEndpointE will return the service access point using the following logic:
//   - For ClusterIP service type, return the URL that maps to ClusterIP and Service Port
//   - For NodePort service type, identify the public IP of the node (if it exists, otherwise return the bound hostname),
//     and the assigned node port for the provided service port, and return the URL that maps to node ip and node port.
//   - For LoadBalancer service type, return the publicly accessible hostname of the load balancer.
//     If the hostname is empty, it will return the public IP of the LoadBalancer.
//   - All other service types are not supported.
func GetServiceEndpointE(t testing.TestingT, options *KubectlOptions, service *corev1.Service, servicePort int) (string, error) {
	return k8s.GetServiceEndpointE(t, options, service, servicePort)
}

// Given the desired servicePort, return the allocated nodeport
func FindNodePortE(service *corev1.Service, servicePort int32) (int32, error) {
	return k8s.FindNodePortE(service, servicePort)
}

// Given a node, return the ip address, preferring the external IP
func FindNodeHostnameE(t testing.TestingT, node corev1.Node) (string, error) {
	return k8s.FindNodeHostnameE(t, node)
}

// ListPods will look for pods in the given namespace that match the given filters and return them. This will fail the
// test if there is an error.
func ListPods(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []corev1.Pod {
	return k8s.ListPods(t, options, filters)
}

// ListPodsE will look for pods in the given namespace that match the given filters and return them.
func ListPodsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]corev1.Pod, error) {
	return k8s.ListPodsE(t, options, filters)
}

// GetPod returns a Kubernetes pod resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetPod(t testing.TestingT, options *KubectlOptions, podName string) *corev1.Pod {
	return k8s.GetPod(t, options, podName)
}

// GetPodE returns a Kubernetes pod resource in the provided namespace with the given name.
func GetPodE(t testing.TestingT, options *KubectlOptions, podName string) (*corev1.Pod, error) {
	return k8s.GetPodE(t, options, podName)
}

// WaitUntilNumPodsCreated waits until the desired number of pods are created that match the provided filter. This will
// retry the check for the specified amount of times, sleeping for the provided duration between each try. This will
// fail the test if the retry times out.
func WaitUntilNumPodsCreated(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions, desiredCount int, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilNumPodsCreated(t, options, filters, desiredCount, retries, sleepBetweenRetries)
}

// WaitUntilNumPodsCreatedE waits until the desired number of pods are created that match the provided filter. This will
// retry the check for the specified amount of times, sleeping for the provided duration between each try.
func WaitUntilNumPodsCreatedE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions, desiredCount int, retries int, sleepBetweenRetries time.Duration) error {
	return k8s.WaitUntilNumPodsCreatedE(t, options, filters, desiredCount, retries, sleepBetweenRetries)
}

// WaitUntilPodAvailable waits until all of the containers within the pod are ready and started, retrying the check for the specified amount of times, sleeping
// for the provided duration between each try. This will fail the test if there is an error or if the check times out.
func WaitUntilPodAvailable(t testing.TestingT, options *KubectlOptions, podName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilPodAvailable(t, options, podName, retries, sleepBetweenRetries)
}

// WaitUntilPodAvailableE waits until all of the containers within the pod are ready and started, retrying the check for the specified amount of times, sleeping
// for the provided duration between each try.
func WaitUntilPodAvailableE(t testing.TestingT, options *KubectlOptions, podName string, retries int, sleepBetweenRetries time.Duration) error {
	return k8s.WaitUntilPodAvailableE(t, options, podName, retries, sleepBetweenRetries)
}

// IsPodAvailable returns true if the all of the containers within the pod are ready and started
func IsPodAvailable(pod *corev1.Pod) bool { return k8s.IsPodAvailable(pod) }

// GetPodLogsE returns the logs of a Pod at the time when the function was called. Pass container name if there are more containers in the Pod or set to "" if there is only one.
// If the Pod is not running an Error is returned.
// If the provided containerName is not the name of a container in the Pod an Error is returned.
func GetPodLogsE(t testing.TestingT, options *KubectlOptions, pod *corev1.Pod, containerName string) (string, error) {
	return k8s.GetPodLogsE(t, options, pod, containerName)
}

// GetPodLogsE returns the logs of a Pod at the time when the function was called.  Pass container name if there are more containers in the Pod or set to "" if there is only one.
func GetPodLogs(t testing.TestingT, options *KubectlOptions, pod *corev1.Pod, containerName string) string {
	return k8s.GetPodLogs(t, options, pod, containerName)
}

// GetClusterRole returns a Kubernetes ClusterRole resource with the given name. This will fail the test if there is an error.
func GetClusterRole(t testing.TestingT, options *KubectlOptions, roleName string) *rbacv1.ClusterRole {
	return k8s.GetClusterRole(t, options, roleName)
}

// GetClusterRoleE returns a Kubernetes ClusterRole resource with the given name.
func GetClusterRoleE(t testing.TestingT, options *KubectlOptions, roleName string) (*rbacv1.ClusterRole, error) {
	return k8s.GetClusterRoleE(t, options, roleName)
}

// ListJobs will look for Jobs in the given namespace that match the given filters and return them. This will fail the
// test if there is an error.
func ListJobs(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []batchv1.Job {
	return k8s.ListJobs(t, options, filters)
}

// ListJobsE will look for jobs in the given namespace that match the given filters and return them.
func ListJobsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]batchv1.Job, error) {
	return k8s.ListJobsE(t, options, filters)
}

// GetJob returns a Kubernetes job resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetJob(t testing.TestingT, options *KubectlOptions, jobName string) *batchv1.Job {
	return k8s.GetJob(t, options, jobName)
}

// GetJobE returns a Kubernetes job resource in the provided namespace with the given name.
func GetJobE(t testing.TestingT, options *KubectlOptions, jobName string) (*batchv1.Job, error) {
	return k8s.GetJobE(t, options, jobName)
}

// WaitUntilJobSucceed waits until requested job is suceeded, retrying the check for the specified amount of times, sleeping
// for the provided duration between each try. This will fail the test if there is an error or if the check times out.
func WaitUntilJobSucceed(t testing.TestingT, options *KubectlOptions, jobName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilJobSucceed(t, options, jobName, retries, sleepBetweenRetries)
}

// WaitUntilJobSucceedE waits until requested job is succeeded, retrying the check for the specified amount of times, sleeping
// for the provided duration between each try.
func WaitUntilJobSucceedE(t testing.TestingT, options *KubectlOptions, jobName string, retries int, sleepBetweenRetries time.Duration) error {
	return k8s.WaitUntilJobSucceedE(t, options, jobName, retries, sleepBetweenRetries)
}

// IsJobSucceeded returns true when the job status condition "Complete" is true. This behavior is documented in the kubernetes API reference:
// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/job-v1/#JobStatus
func IsJobSucceeded(job *batchv1.Job) bool { return k8s.IsJobSucceeded(job) }

// UnmarshalJSONPath allows you to use an arbitrary JSONPath string to query a json blob and unmarshal the resulting
// output into a go object. Note that the output will always be a list. That means that if you query a single object,
// the output will be a list of single element, not the element itself. However, if the json path maps to a list, then
// the output will be that list.
// Example:
//
// jsonBlob := []byte(`{"key": {"data": [1,2,3]}}`)
// jsonPath := "{.key.data[*]}"
// var output []int
// UnmarshalJSONPath(t, jsonBlob, jsonPath, &output)
// // output is []int{1,2,3}
//
// This will fail the test if there is an error.
func UnmarshalJSONPath(t testing.TestingT, jsonData []byte, jsonpathStr string, output interface{}) {
	k8s.UnmarshalJSONPath(t, jsonData, jsonpathStr, output)
}

// UnmarshalJSONPathE allows you to use an arbitrary JSONPath string to query a json blob and unmarshal the resulting
// output into a go object. Note that the output will always be a list. That means that if you query a single object,
// the output will be a list of single element, not the element itself. However, if the json path maps to a list, then
// the output will be that list.
// Example:
//
// jsonBlob := []byte(`{"key": {"data": [1,2,3]}}`)
// jsonPath := "{.key.data[*]}"
// var output []int
// UnmarshalJSONPathE(t, jsonBlob, jsonPath, &output)
// => output = []int{1,2,3}
func UnmarshalJSONPathE(t testing.TestingT, jsonData []byte, jsonpathStr string, output interface{}) error {
	return k8s.UnmarshalJSONPathE(t, jsonData, jsonpathStr, output)
}

// NewKubectlOptions will return a pointer to new instance of KubectlOptions with the configured options
func NewKubectlOptions(contextName string, configPath string, namespace string) *KubectlOptions {
	return k8s.NewKubectlOptions(contextName, configPath, namespace)
}

// NewKubectlOptionsWithInClusterAuth will return a pointer to a new instance of KubectlOptions with the InClusterAuth field set to true
func NewKubectlOptionsWithInClusterAuth() *KubectlOptions {
	return k8s.NewKubectlOptionsWithInClusterAuth()
}

// IsMinikubeE returns true if the underlying kubernetes cluster is Minikube. This is determined by getting the
// associated nodes and checking if all nodes has at least one label namespaced with "minikube.k8s.io".
func IsMinikubeE(t testing.TestingT, options *KubectlOptions) (bool, error) {
	return k8s.IsMinikubeE(t, options)
}

// ListReplicaSets will look for replicasets in the given namespace that match the given filters and return them. This will
// fail the test if there is an error.
func ListReplicaSets(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) []appsv1.ReplicaSet {
	return k8s.ListReplicaSets(t, options, filters)
}

// ListReplicaSetsE will look for replicasets in the given namespace that match the given filters and return them.
func ListReplicaSetsE(t testing.TestingT, options *KubectlOptions, filters metav1.ListOptions) ([]appsv1.ReplicaSet, error) {
	return k8s.ListReplicaSetsE(t, options, filters)
}

// GetReplicaSet returns a Kubernetes replicaset resource in the provided namespace with the given name. This will
// fail the test if there is an error.
func GetReplicaSet(t testing.TestingT, options *KubectlOptions, replicaSetName string) *appsv1.ReplicaSet {
	return k8s.GetReplicaSet(t, options, replicaSetName)
}

// GetReplicaSetE returns a Kubernetes replicaset resource in the provided namespace with the given name.
func GetReplicaSetE(t testing.TestingT, options *KubectlOptions, replicaSetName string) (*appsv1.ReplicaSet, error) {
	return k8s.GetReplicaSetE(t, options, replicaSetName)
}

// CanIDo returns whether or not the provided action is allowed by the client configured by the provided kubectl option.
// This will fail if there are any errors accessing the kubernetes API (but not if the action is denied).
func CanIDo(t testing.TestingT, options *KubectlOptions, action authv1.ResourceAttributes) bool {
	return k8s.CanIDo(t, options, action)
}

// CanIDoE returns whether or not the provided action is allowed by the client configured by the provided kubectl option.
// This will an error if there are problems accessing the kubernetes API (but not if the action is simply denied).
func CanIDoE(t testing.TestingT, options *KubectlOptions, action authv1.ResourceAttributes) (bool, error) {
	return k8s.CanIDoE(t, options, action)
}

// LoadConfigFromPath will load a ClientConfig object from a file path that points to a location on disk containing a
// kubectl config.
func LoadConfigFromPath(path string) clientcmd.ClientConfig { return k8s.LoadConfigFromPath(path) }

// LoadApiClientConfigE will load a ClientConfig object from a file path that points to a location on disk containing a
// kubectl config, with the requested context loaded.
func LoadApiClientConfigE(configPath string, contextName string) (*restclient.Config, error) {
	return k8s.LoadApiClientConfigE(configPath, contextName)
}

// DeleteConfigContextE will remove the context specified at the provided name, and remove any clusters and authinfos
// that are orphaned as a result of it. The config path is either specified in the environment variable KUBECONFIG or at
// the user's home directory under `.kube/config`.
func DeleteConfigContextE(t testing.TestingT, contextName string) error {
	return k8s.DeleteConfigContextE(t, contextName)
}

// DeleteConfigContextWithPathE will remove the context specified at the provided name, and remove any clusters and
// authinfos that are orphaned as a result of it.
func DeleteConfigContextWithPathE(t testing.TestingT, kubeConfigPath string, contextName string) error {
	return k8s.DeleteConfigContextWithPathE(t, kubeConfigPath, contextName)
}

// RemoveOrphanedClusterAndAuthInfoConfig will remove all configurations related to clusters and users that have no
// contexts associated with it
func RemoveOrphanedClusterAndAuthInfoConfig(config *api.Config) {
	k8s.RemoveOrphanedClusterAndAuthInfoConfig(config)
}

// GetKubeConfigPathE determines which file path to use as the kubectl config path
func GetKubeConfigPathE(t testing.TestingT) (string, error) { return k8s.GetKubeConfigPathE(t) }

// KubeConfigPathFromHomeDirE returns a string to the default Kubernetes config path in the home directory. This will
// error if the home directory can not be determined.
func KubeConfigPathFromHomeDirE() (string, error) { return k8s.KubeConfigPathFromHomeDirE() }

// CopyHomeKubeConfigToTemp will copy the kubeconfig in the home directory to a temp file. This will fail the test if
// there are any errors.
func CopyHomeKubeConfigToTemp(t testing.TestingT) string { return k8s.CopyHomeKubeConfigToTemp(t) }

// CopyHomeKubeConfigToTempE will copy the kubeconfig in the home directory to a temp file.
func CopyHomeKubeConfigToTempE(t testing.TestingT) (string, error) {
	return k8s.CopyHomeKubeConfigToTempE(t)
}

// UpsertConfigContext will update or insert a new context to the provided config, binding the provided cluster to the
// provided user.
func UpsertConfigContext(config *api.Config, contextName string, clusterName string, userName string) {
	k8s.UpsertConfigContext(config, contextName, clusterName, userName)
}

// GetConfigMap returns a Kubernetes configmap resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions. This will fail the test if there is an error.
func GetConfigMap(t testing.TestingT, options *KubectlOptions, configMapName string) *corev1.ConfigMap {
	return k8s.GetConfigMap(t, options, configMapName)
}

// GetConfigMapE returns a Kubernetes configmap resource in the provided namespace with the given name. The namespace used
// is the one provided in the KubectlOptions.
func GetConfigMapE(t testing.TestingT, options *KubectlOptions, configMapName string) (*corev1.ConfigMap, error) {
	return k8s.GetConfigMapE(t, options, configMapName)
}

// WaitUntilConfigMapAvailable waits until the configmap is present on the cluster in cases where it is not immediately
// available (for example, when using ClusterIssuer to request a certificate).
func WaitUntilConfigMapAvailable(t testing.TestingT, options *KubectlOptions, configMapName string, retries int, sleepBetweenRetries time.Duration) {
	k8s.WaitUntilConfigMapAvailable(t, options, configMapName, retries, sleepBetweenRetries)
}

// GetServiceAccount returns a Kubernetes service account resource in the provided namespace with the given name. The
// namespace used is the one provided in the KubectlOptions. This will fail the test if there is an error.
func GetServiceAccount(t testing.TestingT, options *KubectlOptions, serviceAccountName string) *corev1.ServiceAccount {
	return k8s.GetServiceAccount(t, options, serviceAccountName)
}

// GetServiceAccountE returns a Kubernetes service account resource in the provided namespace with the given name. The
// namespace used is the one provided in the KubectlOptions.
func GetServiceAccountE(t testing.TestingT, options *KubectlOptions, serviceAccountName string) (*corev1.ServiceAccount, error) {
	return k8s.GetServiceAccountE(t, options, serviceAccountName)
}

// CreateServiceAccount will create a new service account resource in the provided namespace with the given name. The
// namespace used is the one provided in the KubectlOptions. This will fail the test if there is an error.
func CreateServiceAccount(t testing.TestingT, options *KubectlOptions, serviceAccountName string) {
	k8s.CreateServiceAccount(t, options, serviceAccountName)
}

// CreateServiceAccountE will create a new service account resource in the provided namespace with the given name. The
// namespace used is the one provided in the KubectlOptions.
func CreateServiceAccountE(t testing.TestingT, options *KubectlOptions, serviceAccountName string) error {
	return k8s.CreateServiceAccountE(t, options, serviceAccountName)
}

// GetServiceAccountAuthToken will retrieve the ServiceAccount token from the cluster so it can be used to
// authenticate requests as that ServiceAccount. This will fail the test if there is an error.
func GetServiceAccountAuthToken(t testing.TestingT, kubectlOptions *KubectlOptions, serviceAccountName string) string {
	return k8s.GetServiceAccountAuthToken(t, kubectlOptions, serviceAccountName)
}

// GetServiceAccountAuthTokenE will retrieve the ServiceAccount token from the cluster so it can be used to
// authenticate requests as that ServiceAccount.
func GetServiceAccountAuthTokenE(t testing.TestingT, kubectlOptions *KubectlOptions, serviceAccountName string) (string, error) {
	return k8s.GetServiceAccountAuthTokenE(t, kubectlOptions, serviceAccountName)
}

// AddConfigContextForServiceAccountE will add a new config context that binds the ServiceAccount auth token to the
// Kubernetes cluster of the current config context.
func AddConfigContextForServiceAccountE(t testing.TestingT, kubectlOptions *KubectlOptions, contextName string, serviceAccountName string, token string) error {
	return k8s.AddConfigContextForServiceAccountE(t, kubectlOptions, contextName, serviceAccountName, token)
}

// GetKubernetesClusterVersion returns the Kubernetes cluster version.
func GetKubernetesClusterVersionE(t testing.TestingT) (string, error) {
	return k8s.GetKubernetesClusterVersionE(t)
}

// GetKubernetesClusterVersion returns the Kubernetes cluster version given a configured KubectlOptions object.
func GetKubernetesClusterVersionWithOptionsE(t testing.TestingT, kubectlOptions *KubectlOptions) (string, error) {
	return k8s.GetKubernetesClusterVersionWithOptionsE(t, kubectlOptions)
}
