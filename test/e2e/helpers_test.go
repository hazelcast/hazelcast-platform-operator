package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

func GetBackupSequence(t time.Time) string {
	By("Finding Backup sequence")
	logs := InitLogs(t)
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

func InitLogs(t time.Time) io.ReadCloser {
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      hzName + "-0",
		Namespace: hzNamespace,
	}, &corev1.PodLogOptions{
		Follow:    true,
		SinceTime: &metav1.Time{Time: t},
	})
	return logs
}

func CreateHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("Creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})

	lookupKey := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
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

func GetHzClient(ctx context.Context, unisocket bool) *hzClient.Client {
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
	return client
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

func FillTheMapData(ctx context.Context, unisocket bool, mapName string, mapSize int) {
	var m *hzClient.Map
	clientHz := GetHzClient(ctx, unisocket)
	By("using Hazelcast client")
	m, err := clientHz.GetMap(ctx, mapName)
	Expect(err).ToNot(HaveOccurred())
	for i := 0; i < mapSize; i++ {
		_, err = m.Put(ctx, strconv.Itoa(i), strconv.Itoa(i))
		Expect(err).ToNot(HaveOccurred())
	}
	err = clientHz.Shutdown(ctx)
	Expect(err).ToNot(HaveOccurred())

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

func waitForReadyChannel(readyChan chan struct{}, dur time.Duration) error {
	timer := time.NewTimer(dur)
	for {
		select {
		case <-readyChan:
			return nil
		case <-timer.C:
			return fmt.Errorf("timeout waiting for readyChannel")
		}
	}
}
func closeChannel(closeChan chan struct{}) {
	closeChan <- struct{}{}
}

func assertMapStatus(m *hazelcastcomv1alpha1.Map, st hazelcastcomv1alpha1.MapConfigState) *hazelcastcomv1alpha1.Map {
	checkMap := &hazelcastcomv1alpha1.Map{}
	By("Waiting for Map CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.MapConfigState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      m.Name,
				Namespace: m.Namespace,
			}, checkMap)
			if err != nil {
				return ""
			}
			return checkMap.Status.State
		}, timeout, interval).Should(Equal(st))
	})
	return checkMap
}

func getMapConfig(ctx context.Context, client *hzClient.Client, mapName string) codecTypes.MapConfig {
	ci := hzClient.NewClientInternal(client)
	req := codec.EncodeMCGetMapConfigRequest(mapName)
	resp, err := ci.InvokeOnRandomTarget(ctx, req, nil)
	Expect(err).To(BeNil())
	return codec.DecodeMCGetMapConfigResponse(resp)
}

func portForwardPod(sName, sNamespace, port string) (chan struct{}, chan struct{}) {
	defer GinkgoRecover()
	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	Expect(err).To(BeNil())

	roundTripper, upgrader, err := spdy.RoundTripperFor(config)
	Expect(err).To(BeNil())

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", sNamespace, sName)
	hostIP := strings.TrimPrefix(config.Host, "https://")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	forwarder, err := portforward.New(dialer, []string{port}, stopChan, readyChan, out, errOut)
	Expect(err).To(BeNil())

	go func() {
		if err := forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
			GinkgoWriter.Println(err.Error())
			Expect(err).To(BeNil())
		}
	}()
	return stopChan, readyChan
}

func createHazelcastClient(ctx context.Context, h *hazelcastcomv1alpha1.Hazelcast, localPort string) *hzClient.Client {
	config := hzClient.Config{}
	cc := &config.Cluster
	cc.Name = h.Spec.ClusterName
	cc.Network.SetAddresses("localhost:" + localPort)
	client, err := hzClient.StartNewClientWithConfig(ctx, config)
	Expect(err).To(BeNil())
	return client

}

func assertHazelcastRestoreStatus(h *hazelcastcomv1alpha1.Hazelcast, st hazelcastcomv1alpha1.RestoreState) *hazelcastcomv1alpha1.Hazelcast {
	checkHz := &hazelcastcomv1alpha1.Hazelcast{}
	By("Waiting for Map CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.RestoreState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      h.Name,
				Namespace: h.Namespace,
			}, checkHz)
			if err != nil {
				return ""
			}
			return checkHz.Status.Restore.State
		}, timeout, interval).Should(Equal(st))
	})
	return checkHz
}

func assertMapConfigsPersisted(hazelcast *hazelcastcomv1alpha1.Hazelcast, maps ...string) *config.HazelcastWrapper {
	cm := &corev1.ConfigMap{}
	returnConfig := &config.HazelcastWrapper{}
	Eventually(func() []string {
		hzConfig := &config.HazelcastWrapper{}
		err := k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      hazelcast.Name,
			Namespace: hazelcast.Namespace,
		}, cm)
		if err != nil {
			return nil
		}
		err = yaml.Unmarshal([]byte(cm.Data["hazelcast.yaml"]), hzConfig)
		if err != nil {
			return nil
		}
		keys := make([]string, 0, len(hzConfig.Hazelcast.Map))
		for k := range hzConfig.Hazelcast.Map {
			keys = append(keys, k)
		}

		returnConfig = hzConfig
		return keys

	}, timeout, interval).Should(ConsistOf(maps))

	return returnConfig
}
