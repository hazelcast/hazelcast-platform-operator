package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	. "time"

	hzclienttypes "github.com/hazelcast/hazelcast-go-client/types"
	"k8s.io/apimachinery/pkg/api/resource"

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
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

func GetBackupSequence(t Time, lk types.NamespacedName) string {
	By("Finding Backup sequence")
	logs := InitLogs(t, lk)
	scanner := bufio.NewScanner(logs)
	test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("Starting new hot backup with sequence"))
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

func InitLogs(t Time, lk types.NamespacedName) io.ReadCloser {
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      lk.Name + "-0",
		Namespace: lk.Namespace,
	}, &corev1.PodLogOptions{
		Follow:    true,
		SinceTime: &metav1.Time{Time: t},
		Container: "hazelcast",
	})
	return logs
}

func CreateHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})
	lk := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
	By("checking Hazelcast CR running", func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), lk, hz)
			Expect(err).ToNot(HaveOccurred())
			return isHazelcastRunning(hz)
		}, 10*Minute, interval).Should(BeTrue())
	})
}

func CreateHazelcastCRWithoutCheck(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})
}

func RemoveHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	Expect(k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
	assertDoesNotExist(types.NamespacedName{
		Name:      hazelcast.Name + "-0",
		Namespace: hazelcast.Namespace,
	}, &corev1.Pod{})

	By("waiting for Hazelcast CR to be removed", func() {
		Eventually(func() error {
			h := &hazelcastcomv1alpha1.Hazelcast{}
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      hazelcast.Name,
				Namespace: hazelcast.Namespace,
			}, h)
		}, 1*Minute, interval).ShouldNot(Succeed())
	})
}
func DeletePod(podName string, gracePeriod int64) {
	log.Printf("deleting POD with name '%s'", podName)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}
	err := GetClientSet().CoreV1().Pods(hzNamespace).Delete(context.Background(), podName, deleteOptions)
	if err != nil {
		log.Fatal(err)
	}
}

func GetHzClient(ctx context.Context, lk types.NamespacedName, unisocket bool) *hzClient.Client {
	s := &corev1.Service{}
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), lk, s)
		Expect(err).ToNot(HaveOccurred())
		return len(s.Status.LoadBalancer.Ingress) > 0
	}, 3*Minute, interval).Should(BeTrue())
	addr := s.Status.LoadBalancer.Ingress[0].IP
	if addr == "" {
		addr = s.Status.LoadBalancer.Ingress[0].Hostname
	}
	Expect(addr).Should(Not(BeEmpty()))

	hz := &hazelcastcomv1alpha1.Hazelcast{}
	Expect(k8sClient.Get(context.Background(), lk, hz)).Should(Succeed())
	clusterName := "dev"
	if len(hz.Spec.ClusterName) > 0 {
		clusterName = hz.Spec.ClusterName
	}

	By("connecting Hazelcast client")
	config := hzClient.Config{}
	config.Cluster.Network.SetAddresses(fmt.Sprintf("%s:5701", addr))
	config.Cluster.Unisocket = unisocket
	config.Cluster.Name = clusterName
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

func FillTheMapData(ctx context.Context, lk types.NamespacedName, unisocket bool, mapName string, mapSize int) {
	var m *hzClient.Map
	clientHz := GetHzClient(ctx, lk, unisocket)
	By("using Hazelcast client")
	m, err := clientHz.GetMap(ctx, mapName)
	Expect(err).ToNot(HaveOccurred())
	entries := make([]hzclienttypes.Entry, 0, mapSize)
	for i := 0; i < mapSize; i++ {
		entries = append(entries, hzclienttypes.NewEntry(strconv.Itoa(i), strconv.Itoa(i)))
	}
	err = m.PutAll(ctx, entries...)
	Expect(err).ToNot(HaveOccurred())
	err = clientHz.Shutdown(ctx)
	Expect(err).ToNot(HaveOccurred())
}

func waitForMapSize(ctx context.Context, lk types.NamespacedName, mapName string, mapSize int) {
	var m *hzClient.Map
	clientHz := GetHzClient(ctx, lk, true)
	By("using Hazelcast client")
	m, err := clientHz.GetMap(ctx, mapName)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func() (int, error) {
		return m.Size(ctx)
	}, 2*Minute, interval).Should(Equal(mapSize))
}

func FillTheMapWithHugeData(ctx context.Context, mapName string, mapSizeInGb string, hzConfig *hazelcastcomv1alpha1.Hazelcast) {
	hzAddress := fmt.Sprintf("%s.%s.svc.cluster.local:%d", hzConfig.Name, hzConfig.Namespace, n.DefaultHzPort)
	var m *hzClient.Map
	clientPod := CreateClientPod(hzAddress, mapSizeInGb, mapName)
	defer DeletePod(clientPod.Name, 0)
	mapSize, _ := strconv.ParseFloat(mapSizeInGb, 64)
	client := GetHzClient(ctx, types.NamespacedName{Name: hzConfig.Name, Namespace: hzConfig.Namespace}, false)
	m, _ = client.GetMap(ctx, mapName)
	Eventually(func() (int, error) {
		return m.Size(ctx)
	}, 15*Minute, interval).Should(Equal(int(math.Round(mapSize*1310.72) * 100)))
	// 1310.72 entries per one Go routine. Formula: 1073741824 Bytes per 1Gb  / 8192 Bytes per entry / 100 go routines
	err := client.Shutdown(ctx)
	Expect(err).ToNot(HaveOccurred())
}

func CreateClientPod(hzAddress string, mapSizeInGb string, mapName string) *corev1.Pod {
	clientPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"client": "true",
			},
			Name: "client-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "client-container",
					Image: "cheels/docker-backup:latest",
					Args:  []string{"/fill_map", "-address", hzAddress, "-size", mapSizeInGb, "-mapName", mapName},
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse(mapSizeInGb + "Gi")}},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	_, err := GetClientSet().CoreV1().Pods(hzNamespace).Create(context.Background(), clientPod, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      clientPod.Name,
		Namespace: hzNamespace,
	}, clientPod)
	Expect(err).ToNot(HaveOccurred())
	Eventually(func() bool {
		pod, err := GetClientSet().CoreV1().Pods(hzNamespace).Get(context.Background(), clientPod.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		return pod.Status.ContainerStatuses[0].Ready
	}, 5*Minute, interval).Should(Equal(true))
	return clientPod
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
	}, 6*Minute, interval).Should(Equal(fmt.Sprintf("%d/%d", membersCount, membersCount)))
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

func waitForReadyChannel(readyChan chan struct{}, dur Duration) error {
	timer := NewTimer(dur)
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
	By("waiting for Map CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.MapConfigState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      m.Name,
				Namespace: m.Namespace,
			}, checkMap)
			if err != nil {
				return ""
			}
			return checkMap.Status.State
		}, 20*Second, interval).Should(Equal(st))
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

func isManagementCenterRunning(mc *hazelcastcomv1alpha1.ManagementCenter) bool {
	return mc.Status.Phase == "Running"
}

func assertHazelcastRestoreStatus(h *hazelcastcomv1alpha1.Hazelcast, st hazelcastcomv1alpha1.RestoreState) *hazelcastcomv1alpha1.Hazelcast {
	checkHz := &hazelcastcomv1alpha1.Hazelcast{}
	By("waiting for Map CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.RestoreState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      h.Name,
				Namespace: h.Namespace,
			}, checkHz)
			if err != nil {
				return ""
			}
			if checkHz.Status.Restore == nil {
				return ""
			}
			return checkHz.Status.Restore.State
		}, 20*Second, interval).Should(Equal(st))
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
	}, 20*Second, interval).Should(ConsistOf(maps))
	return returnConfig
}
