package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/go-resty/resty/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	types2 "github.com/onsi/gomega/types"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	hzClient "github.com/hazelcast/hazelcast-go-client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/platform"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

func GetBackupSequence() string {
	By("Finding Backup sequence")
	sinceSecond := int64(360)
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      hzName + "-0",
		Namespace: hzNamespace,
	}, &corev1.PodLogOptions{
		Follow:       true,
		SinceSeconds: &sinceSecond,
	})
	defer logs.Close()
	scanner := bufio.NewScanner(logs)
	test.EventuallyInLogs(scanner, timeout, logInterval).Should(ContainSubstring("Starting new hot backup with sequence"))
	line := scanner.Text()
	Expect(logs.Close()).Should(Succeed())

	compRegEx := regexp.MustCompile(`Starting new hot backup with sequence (?P<seq>\d+)`)
	match := compRegEx.FindStringSubmatch(line)
	var seq string
	for i, name := range compRegEx.SubexpNames() {
		if name == "seq" && i > 0 && i <= len(match) {
			seq = match[i]
		}
	}
	if seq == "" {
		Fail("Backup sequence not found")
	}
	return seq
}

func ReadLogs(scanner *bufio.Scanner, matcher types2.GomegaMatcher) {
	test.EventuallyInLogs(scanner, timeout, logInterval).Should(matcher)
}

func InitLogs() io.ReadCloser {
	sinceSecond := int64(360)
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      hzName + "-0",
		Namespace: hzNamespace,
	}, &corev1.PodLogOptions{
		Follow:       true,
		SinceSeconds: &sinceSecond,
	})
	return logs
}

func CreateHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast, lookupKey types.NamespacedName) {
	By("Creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})

	By("Checking Hazelcast CR running", func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), lookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			return isHazelcastRunning(hz)
		}, timeout, interval).Should(BeTrue())
	})
}

func RemoveHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	Expect(k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
	assertDoesNotExist(types.NamespacedName{
		Name:      hzName + "-0",
		Namespace: hzNamespace,
	}, &corev1.Pod{})

	By("Waiting for Hazelcast CR to be removed", func() {
		Eventually(func() error {
			h := &hazelcastcomv1alpha1.Hazelcast{}
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      hzName,
				Namespace: hzNamespace,
			}, h)
		}, timeout, interval).ShouldNot(Succeed())
	})
}
func DeletePod(podName string, gracePeriod int64) {
	log.Printf("Deleting POD with name '%s'", podName)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}
	err := GetClientSet().CoreV1().Pods(hzNamespace).Delete(context.Background(), podName, deleteOptions)
	if err != nil {
		log.Fatal(err)
	}
}

func GetHzClient(ctx context.Context, unisocket bool) (*hzClient.Client, error) {
	var lookupKey = types.NamespacedName{
		Name:      hzName,
		Namespace: hzNamespace,
	}
	s := &corev1.Service{}
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), lookupKey, s)
		Expect(err).ToNot(HaveOccurred())
		return len(s.Status.LoadBalancer.Ingress) > 0
	}, timeout, interval).Should(BeTrue())
	addr := s.Status.LoadBalancer.Ingress[0].IP
	if addr == "" {
		addr = s.Status.LoadBalancer.Ingress[0].Hostname
	}
	Expect(addr).Should(Not(BeEmpty()))
	By("connecting Hazelcast client")
	config := hzClient.Config{}
	config.Cluster.Network.SetAddresses(fmt.Sprintf("%s:5701", addr))
	config.Cluster.Unisocket = unisocket
	config.Cluster.Discovery.UsePublicIP = true
	client, err := hzClient.StartNewClientWithConfig(ctx, config)
	Expect(err).ToNot(HaveOccurred())
	return client, err
}

func GetClientSet() *kubernetes.Clientset {
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatal(err)
	}
	return clientSet
}

func ChangeHzClusterState(clusterState string, interval, timeout time.Duration) string {
	log.Printf("Changing the cluster state to '%s'", clusterState)
	clientSet := GetClientSet()
	clusterState = strings.ToUpper(clusterState)
	service, err := clientSet.CoreV1().Services(hzNamespace).Get(context.Background(), GetServiceName(), metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}
	loadBalancerStatus := service.Status.LoadBalancer.Ingress
	if len(loadBalancerStatus) == 0 {
		log.Fatalf("Cluster is running without next param: 'service.type=LoadBalancer' ")
	}
	response, err := RestClient().
		SetBody("dev&&" + clusterState).
		Post("http://" + loadBalancerStatus[0].IP + ":5701/hazelcast/rest/management/cluster/changeState")
	if err != nil {
		log.Fatal(err)
	}
	if err := wait.Poll(interval, timeout, func() (bool, error) {
		return getHzClusterState() == clusterState, nil
	}); err != nil {
		log.Panicf("Error waiting for cluster state to reach expected state: %v. Expected %v, had %v", err, clusterState, getHzClusterState())
		Expect(err).ToNot(HaveOccurred())
	}
	return gjson.Parse(response.String()).Get("status").Raw
}

func getHzClusterState() string {
	log.Print("Getting the cluster state")
	clientSet := GetClientSet()
	service, err := clientSet.CoreV1().Services(hzNamespace).Get(context.Background(), GetServiceName(), metav1.GetOptions{})
	if err != nil {
		log.Panic(err)
	}
	loadBalancerStatus := service.Status.LoadBalancer.Ingress
	if len(loadBalancerStatus) == 0 {
		log.Fatal("Cluster is running without next param: 'service.type=LoadBalancer' ")
	}
	response, err := RestClient().
		Get("http://" + loadBalancerStatus[0].IP + ":5701/hazelcast/health/cluster-state")
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Cluster state is '%s'", strings.ReplaceAll(response.String(), "\"", ""))
	return strings.ReplaceAll(response.String(), "\"", "")
}

func FillTheMapData(ctx context.Context, unisocket bool, mapName string, mapSize int) {
	var m *hzClient.Map
	clientHz, err := GetHzClient(ctx, unisocket)
	Expect(err).ToNot(HaveOccurred())
	By("using Hazelcast client")
	m, err = clientHz.GetMap(ctx, mapName)
	Expect(err).ToNot(HaveOccurred())
	for i := 0; i < mapSize; i++ {
		_, err = m.Put(ctx, strconv.Itoa(i), strconv.Itoa(i))
		Expect(err).ToNot(HaveOccurred())
	}
	err = clientHz.Shutdown(ctx)
	Expect(err).ToNot(HaveOccurred())

}

func RestClient() *resty.Request {
	client := resty.New()
	request := client.R().EnableTrace()
	return request
}

func GetServiceName() string {
	serviceName, _ := ExecuteBashCommand("kubectl get service --field-selector=metadata.name!=kubernetes --no-headers=true  |  awk {'print $1'}")
	return serviceName[0]
}
func ExecuteBashCommand(commandToExecute string) (stdout, stderr []string) {
	command := cmd.NewCmd("bash", "-c", commandToExecute)
	log.Printf("Executing bash command '%s'", commandToExecute)
	s := <-command.Start()
	stdout = s.Stdout
	stderr = s.Stderr
	for _, out := range stdout {
		fmt.Println(out)
	}
	for _, err := range stderr {
		if err != "" {
			log.Panic(err)
		}
	}
	return stdout, stderr
}

func emptyHazelcast() *hazelcastcomv1alpha1.Hazelcast {
	return &hazelcastcomv1alpha1.Hazelcast{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hzName,
			Namespace: hzNamespace,
		},
	}
}

func isHazelcastRunning(hz *hazelcastcomv1alpha1.Hazelcast) bool {
	return hz.Status.Phase == "Running"
}

// assertMemberLogs check that the given expected string can be found in the logs.
// expected can be a regexp pattern.
func assertMemberLogs(h *hazelcastcomv1alpha1.Hazelcast, expected string) {
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      h.Name + "-0",
		Namespace: h.Namespace,
	}, &corev1.PodLogOptions{})
	defer logs.Close()
	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		println(line)
		if match, _ := regexp.MatchString(expected, line); match {
			return
		}
	}
	Fail(fmt.Sprintf("Failed to find \"%s\" in member logs", expected))
}

func evaluateReadyMembers(lookupKey types.NamespacedName, membersCount int) {
	hz := &hazelcastcomv1alpha1.Hazelcast{}
	Eventually(func() string {
		err := k8sClient.Get(context.Background(), lookupKey, hz)
		Expect(err).ToNot(HaveOccurred())
		return hz.Status.Cluster.ReadyMembers
	}, timeout, interval).Should(Equal(fmt.Sprintf("%d/%d", membersCount, membersCount)))
}

func getFirstWorkerNodeName() string {
	labelMatcher := client.MatchingLabels{}
	if platform.GetPlatform().Type == platform.OpenShift {
		labelMatcher = client.MatchingLabels{
			"node-role.kubernetes.io/worker": "",
		}
	}
	nodes := &corev1.NodeList{}
	Expect(k8sClient.List(context.Background(), nodes, labelMatcher)).Should(Succeed())
loop1:
	for _, node := range nodes.Items {
		for _, taint := range node.Spec.Taints {
			if taint.Key == "node.kubernetes.io/unreachable" {
				continue loop1
			}
		}
		return node.ObjectMeta.Name
	}
	Fail("Could not find a reachable working node.")
	return ""
}

func addNodeSelectorForName(hz *hazelcastcomv1alpha1.Hazelcast, n string) *hazelcastcomv1alpha1.Hazelcast {
	// If hostPath is not enabled, do nothing
	if hz.Spec.Scheduling == nil {
		return hz
	}
	// If NodeSelector is set with dummy name, put the real node name
	if hz.Spec.Scheduling.NodeSelector != nil {
		hz.Spec.Scheduling.NodeSelector = map[string]string{"kubernetes.io/hostname": n}
	}
	return hz
}

func assertPersistenceVolumeExist(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	pods := &corev1.PodList{}
	podLabels := client.MatchingLabels{
		n.ApplicationNameLabel:         n.Hazelcast,
		n.ApplicationInstanceNameLabel: hazelcast.Name,
		n.ApplicationManagedByLabel:    n.OperatorName,
	}
	if err := k8sClient.List(context.Background(), pods, client.InNamespace(hazelcast.Namespace), podLabels); err != nil {
		Fail("Could not find Pods for Hazelcast " + hazelcast.Name)
	}

	for _, pod := range pods.Items {
		Expect(pod.Spec.Volumes).Should(ContainElement(corev1.Volume{
			Name: n.PersistenceVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: n.PersistenceVolumeName + "-" + pod.Name,
				},
			},
		}))
	}
}
