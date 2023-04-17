package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	hazelcastv1beta1 "github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	. "time"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	"github.com/hazelcast/hazelcast-go-client/logger"
	hzclienttypes "github.com/hazelcast/hazelcast-go-client/types"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	ginkgoTypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
)

type UpdateFn func(*hazelcastv1beta1.Hazelcast) *hazelcastv1beta1.Hazelcast

func GetBackupSequence(t Time, lk types.NamespacedName) string {
	var seq string
	By("finding Backup sequence", func() {
		logs := InitLogs(t, lk)
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		test.EventuallyInLogs(logReader, 10*Second, logInterval).Should(ContainSubstring("Starting new hot backup with sequence"))
		line := logReader.History[len(logReader.History)-1]
		compRegEx := regexp.MustCompile(`Starting new hot backup with sequence (?P<seq>\d+)`)
		match := compRegEx.FindStringSubmatch(line)

		for i, name := range compRegEx.SubexpNames() {
			if name == "seq" && i > 0 && i <= len(match) {
				seq = match[i]
			}
		}
		if seq == "" {
			Fail("Backup sequence not found")
		}
	})
	return seq
}

func InitLogs(t Time, lk types.NamespacedName) io.ReadCloser {
	var logs io.ReadCloser
	By("getting Hazelcast logs", func() {
		logs = test.GetPodLogs(context.Background(), types.NamespacedName{
			Name:      lk.Name + "-0",
			Namespace: lk.Namespace,
		}, &corev1.PodLogOptions{
			Follow:    true,
			SinceTime: &metav1.Time{Time: t},
			Container: "hazelcast",
		})
	})
	return logs
}

func SidecarAgentLogs(t Time, lk types.NamespacedName) io.ReadCloser {
	var logs io.ReadCloser
	By("getting sidecar agent logs", func() {
		logs = test.GetPodLogs(context.Background(), types.NamespacedName{
			Name:      lk.Name + "-0",
			Namespace: lk.Namespace,
		}, &corev1.PodLogOptions{
			Follow:    true,
			SinceTime: &metav1.Time{Time: t},
			Container: "sidecar-agent",
		})
	})
	return logs
}

func CreateHazelcastCR(hazelcast *hazelcastv1beta1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})
	lk := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
	message := ""
	By("checking Hazelcast CR running", func() {
		hz := &hazelcastv1beta1.Hazelcast{}
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), lk, hz)
			if err != nil {
				return false
			}
			message = hz.Status.Message
			return isHazelcastRunning(hz)
		}, 10*Minute, interval).Should(BeTrue(), "Message: %v", message)
	})
}

func UpdateHazelcastCR(hazelcast *hazelcastv1beta1.Hazelcast, fns ...UpdateFn) {
	By("updating the CR", func() {
		if len(fns) == 0 {
			Expect(k8sClient.Update(context.Background(), hazelcast)).Should(Succeed())
		} else {
			lk := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
			for {
				cr := &hazelcastv1beta1.Hazelcast{}
				Expect(k8sClient.Get(context.Background(), lk, cr)).Should(Succeed())
				for _, fn := range fns {
					cr = fn(cr)
				}
				err := k8sClient.Update(context.Background(), cr)
				if err == nil {
					break
				} else if errors.IsConflict(err) {
					continue
				} else {
					Fail(err.Error())
				}
			}
		}
	})
}

func CreateHazelcastCRWithoutCheck(hazelcast *hazelcastv1beta1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
	})
}

func RemoveHazelcastCR(hazelcast *hazelcastv1beta1.Hazelcast) {
	By("removing hazelcast CR", func() {
		Expect(k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
		assertDoesNotExist(types.NamespacedName{
			Name:      hazelcast.Name + "-0",
			Namespace: hazelcast.Namespace,
		}, &corev1.Pod{})
	})
	By("waiting for Hazelcast CR to be removed", func() {
		Eventually(func() error {
			h := &hazelcastv1beta1.Hazelcast{}
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      hazelcast.Name,
				Namespace: hazelcast.Namespace,
			}, h)
		}, 2*Minute, interval).ShouldNot(Succeed())
	})
}
func DeletePod(podName string, gracePeriod int64, lk types.NamespacedName) {
	By(fmt.Sprintf("deleting POD with name '%s'", podName), func() {
		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		}
		err := getClientSet().CoreV1().Pods(lk.Namespace).Delete(context.Background(), podName, deleteOptions)
		if err != nil {
			log.Fatal(err)
		}
	})
}

func WaitForPodReady(podName string, lk types.NamespacedName, timeout time.Duration) {
	watcher, err := getClientSet().CoreV1().Pods(lk.Namespace).Watch(context.Background(), metav1.ListOptions{FieldSelector: "metadata.name=" + podName})
	if err != nil {
		log.Fatalf("failed to create pod watcher: %v", err)
	}
	defer watcher.Stop()
	timeoutCh := time.After(timeout)

	for {
		select {
		case event, eventOk := <-watcher.ResultChan():
			if !eventOk {
				log.Fatal("watch channel closed before pod became ready")
			}
			if event.Type == watch.Modified {
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					log.Fatalf("unexpected object type: %v", event.Object)
				}
				for _, c := range pod.Status.Conditions {
					if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
						return
					}
				}
			}
		case <-timeoutCh:
			watcher.Stop()
			log.Fatalf("timed out waiting for pod %s to become ready in namespace %s", podName, lk.Namespace)
		}
	}
}

func GetHzClient(ctx context.Context, lk types.NamespacedName, unisocket bool) *hzClient.Client {
	clientWithConfig := &hzClient.Client{}
	By("starting new Hazelcast client", func() {
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
		c := hzClient.Config{}
		c.Labels = []string{"e2e-test=true"}
		c.Cluster.Network.SetAddresses(fmt.Sprintf("%s:5701", addr))
		c.Cluster.Unisocket = unisocket
		c.Cluster.Name = clusterName
		c.Cluster.Discovery.UsePublicIP = true
		Eventually(func() *hzClient.Client {
			clientWithConfig, _ = hzClient.StartNewClientWithConfig(ctx, c)
			return clientWithConfig
		}, 3*Minute, interval).Should(Not(BeNil()))
	})
	return clientWithConfig
}

func getClientSet() *kubernetes.Clientset {
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

func SwitchContext(context string) {
	By(fmt.Sprintf("switch to '%s' context", context), func() {
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
		rawConfig, err := kubeConfig.RawConfig()
		if err != nil {
			log.Fatal(err)
		}
		if rawConfig.Contexts[context] == nil {
			log.Fatalf("Specified context %v doesn't exists. Please check you default kubeconfig path", context)
		}
		rawConfig.CurrentContext = context
		err = clientcmd.ModifyConfig(clientcmd.NewDefaultPathOptions(), rawConfig, true)
		if err != nil {
			log.Fatal(err)
		}
	})
}
func FillTheMapData(ctx context.Context, lk types.NamespacedName, unisocket bool, mapName string, entryCount int) {
	By(fmt.Sprintf("filling the '%s' map with '%d' entries using '%s' lookup name and '%s' namespace", mapName, entryCount, lk.Name, lk.Namespace), func() {
		var m *hzClient.Map
		clientHz := GetHzClient(ctx, lk, unisocket)
		m, err := clientHz.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		initMapSize, err := m.Size(ctx)
		Expect(err).ToNot(HaveOccurred())
		entries := make([]hzclienttypes.Entry, 0, entryCount)
		for i := initMapSize; i < initMapSize+entryCount; i++ {
			entries = append(entries, hzclienttypes.NewEntry(strconv.Itoa(i), strconv.Itoa(i)))
		}
		err = m.PutAll(ctx, entries...)
		Expect(err).ToNot(HaveOccurred())
		mapSize, err := m.Size(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(mapSize).To(Equal(initMapSize + entryCount))
		err = clientHz.Shutdown(ctx)
		Expect(err).ToNot(HaveOccurred())
	})
}

func WaitForMapSize(ctx context.Context, lk types.NamespacedName, mapName string, mapSize int, timeout Duration) {
	By(fmt.Sprintf("waiting the '%s' map to be of size '%d' using lookup name '%s'", mapName, mapSize, lk.Name), func() {
		if timeout == 0 {
			timeout = 10 * Minute
		}
		clientHz := GetHzClient(ctx, lk, true)
		defer func() {
			err := clientHz.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()
		Eventually(func() (int, error) {
			hzMap, err := clientHz.GetMap(ctx, mapName)
			if err != nil {
				return -1, err
			}
			return hzMap.Size(ctx)
		}, timeout, 5*Second).Should(Equal(mapSize))
	})
}

/*
2 (entries per single goroutine) = 1048576 (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
*/

func FillTheMapWithData(ctx context.Context, mapName string, sizeInMb int, hzConfig *hazelcastv1beta1.Hazelcast) {
	By(fmt.Sprintf("filling the map '%s' with '%d' MB data", mapName, sizeInMb), func() {
		hzAddress := hzclient.HazelcastUrl(hzConfig)
		clientHz := GetHzClient(ctx, types.NamespacedName{Name: hzConfig.Name, Namespace: hzConfig.Namespace}, true)
		defer func() {
			err := clientHz.Shutdown(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
		mapLoaderPod := createMapLoaderPod(hzAddress, hzConfig.Spec.ClusterName, sizeInMb, mapName, types.NamespacedName{Name: hzConfig.Name, Namespace: hzConfig.Namespace})
		defer DeletePod(mapLoaderPod.Name, 0, types.NamespacedName{Namespace: hzConfig.Namespace})
		Eventually(func() int {
			return countKeySet(ctx, clientHz, mapName, hzConfig)
		}, 15*Minute, interval).Should(Equal(sizeInMb * 128)) // 128 entries/Mb = 2 (entries) * 64 (goroutines)
	})
}

func countKeySet(ctx context.Context, clientHz *hzClient.Client, mapName string, hzConfig *hazelcastv1beta1.Hazelcast) int {
	keyCount := 0
	m, _ := clientHz.GetMap(ctx, mapName)
	keySet, _ := m.GetKeySet(ctx)
	for _, key := range keySet {
		if strings.HasPrefix(fmt.Sprint(key), hzConfig.Spec.ClusterName) {
			keyCount++
		}
	}
	return keyCount
}

func createMapLoaderPod(hzAddress, clusterName string, mapSizeInMb int, mapName string, lk types.NamespacedName) *corev1.Pod {
	size := strconv.Itoa(mapSizeInMb)
	clientPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"maploader": "true",
			},
			Name:      "maploader-" + lk.Name,
			Namespace: lk.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "maploader-container",
					Image: mapLoaderImage(),
					Args: []string{
						"/maploader",
						"-address", hzAddress,
						"-clusterName", clusterName,
						"-size", size,
						"-mapName", mapName,
					},
					ImagePullPolicy: corev1.PullAlways,
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	_, err := getClientSet().CoreV1().Pods(lk.Namespace).Create(context.Background(), clientPod, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      clientPod.Name,
		Namespace: lk.Namespace,
	}, clientPod)
	Expect(err).ToNot(HaveOccurred())
	return clientPod
}

func mapLoaderImage() string {
	img, ok := os.LookupEnv("MAPLOADER_IMAGE")
	if ok {
		return img
	}
	return "us-east1-docker.pkg.dev/hazelcast-33/hazelcast-platform-operator/wan-replication-maploader:700d"
}

func isHazelcastRunning(hz *hazelcastv1beta1.Hazelcast) bool {
	return hz.Status.Phase == "Running"
}

// assertMemberLogs check that the given expected string can be found in the logs.
// expected can be a regexp pattern.
func assertMemberLogs(h *hazelcastv1beta1.Hazelcast, expected string) {
	logs := test.GetPodLogs(context.Background(), types.NamespacedName{
		Name:      h.Name + "-0",
		Namespace: h.Namespace,
	}, &corev1.PodLogOptions{
		Container: "hazelcast",
	})
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

func evaluateReadyMembers(lookupKey types.NamespacedName) {
	By(fmt.Sprintf("evaluate number of ready members for lookup name '%s' and '%s' namespace", lookupKey.Name, lookupKey.Namespace), func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		err := k8sClient.Get(context.Background(), lookupKey, hz)
		Expect(err).ToNot(HaveOccurred())
		membersCount := int(*hz.Spec.ClusterSize)
		Eventually(func() string {
			err := k8sClient.Get(context.Background(), lookupKey, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.Cluster.ReadyMembers
		}, 6*Minute, interval).Should(Equal(fmt.Sprintf("%d/%d", membersCount, membersCount)))
	})
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
		}, 40*Second, interval).Should(Equal(st))
	})
	return checkMap
}

func assertWanStatus(wr *hazelcastcomv1alpha1.WanReplication, st hazelcastcomv1alpha1.WanStatus) *hazelcastcomv1alpha1.WanReplication {
	checkWan := &hazelcastcomv1alpha1.WanReplication{}
	By("waiting for WAN CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.WanStatus {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      wr.Name,
				Namespace: wr.Namespace,
			}, checkWan)
			if err != nil {
				return ""
			}
			return checkWan.Status.Status
		}, 1*Minute, interval).Should(Equal(st))
	})
	return checkWan
}

func assertWanStatusMapCount(wr *hazelcastcomv1alpha1.WanReplication, mapLen int) *hazelcastcomv1alpha1.WanReplication {
	checkWan := &hazelcastcomv1alpha1.WanReplication{}
	By("waiting for WAN CR status map length", func() {
		Eventually(func() int {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      wr.Name,
				Namespace: wr.Namespace,
			}, checkWan)
			if err != nil {
				return -1
			}
			return len(checkWan.Status.WanReplicationMapsStatus)
		}, 1*Minute, interval).Should(Equal(mapLen))
	})
	return checkWan
}

func getMemberConfig(ctx context.Context, client *hzClient.Client) string {
	ci := hzClient.NewClientInternal(client)
	req := codec.EncodeMCGetMemberConfigRequest()
	resp, err := ci.InvokeOnRandomTarget(ctx, req, nil)
	Expect(err).To(BeNil())
	return codec.DecodeMCGetMemberConfigResponse(resp)
}

func getMapConfig(ctx context.Context, client *hzClient.Client, mapName string) codecTypes.MapConfig {
	ci := hzClient.NewClientInternal(client)
	req := codec.EncodeMCGetMapConfigRequest(mapName)
	resp, err := ci.InvokeOnRandomTarget(ctx, req, nil)
	Expect(err).To(BeNil())
	return codec.DecodeMCGetMapConfigResponse(resp)
}

func portForwardPod(sName, sNamespace, port string) chan struct{} {
	defer GinkgoRecover()
	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	clientConfig, err := kubeConfig.ClientConfig()
	Expect(err).To(BeNil())

	roundTripper, upgrader, err := spdy.RoundTripperFor(clientConfig)
	Expect(err).To(BeNil())

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", sNamespace, sName)
	hostIP := strings.TrimPrefix(clientConfig.Host, "https://")
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

	err = waitForReadyChannel(readyChan, 5*Second)
	Expect(err).To(BeNil())
	return stopChan
}

func newHazelcastClientPortForward(ctx context.Context, h *hazelcastv1beta1.Hazelcast, localPort string) *hzClient.Client {
	clientWithConfig := &hzClient.Client{}
	By(fmt.Sprintf("creating Hazelcast client using address '%s'", "localhost:"+localPort), func() {
		c := hzClient.Config{
			Logger: logger.Config{
				Level: logger.DebugLevel,
			},
			Cluster: cluster.Config{
				Unisocket: true,
				Name:      h.Spec.ClusterName,
				ConnectionStrategy: cluster.ConnectionStrategyConfig{
					Timeout:       hztypes.Duration(2 * time.Second),
					ReconnectMode: cluster.ReconnectModeOff,
				},
			},
		}
		c.Cluster.Network.SetAddresses("localhost:" + localPort)
		Eventually(func() (err error) {
			clientWithConfig, err = hzClient.StartNewClientWithConfig(ctx, c)
			return err
		}, 3*Minute, interval).Should(BeNil())
	})

	return clientWithConfig
}

func isManagementCenterRunning(mc *hazelcastcomv1alpha1.ManagementCenter) bool {
	return mc.Status.Phase == "Running"
}

func assertHazelcastRestoreStatus(h *hazelcastv1beta1.Hazelcast, st hazelcastcomv1alpha1.RestoreState) *hazelcastv1beta1.Hazelcast {
	checkHz := &hazelcastv1beta1.Hazelcast{}
	By("waiting for Map CR status", func() {
		Eventually(func() hazelcastv1beta1.RestoreState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      h.Name,
				Namespace: h.Namespace,
			}, checkHz)
			if err != nil {
				return ""
			}
			if checkHz.Status.Restore == (hazelcastv1beta1.RestoreStatus{}) {
				return ""
			}
			return checkHz.Status.Restore.State
		}, 40*Second, interval).Should(Equal(st))
	})
	return checkHz
}

func assertCacheConfigsPersisted(hazelcast *hazelcastv1beta1.Hazelcast, caches ...string) *config.HazelcastWrapper {
	cm := &corev1.Secret{}
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
		err = yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
		if err != nil {
			return nil
		}
		keys := make([]string, 0, len(hzConfig.Hazelcast.Cache))
		for k := range hzConfig.Hazelcast.Cache {
			keys = append(keys, k)
		}
		returnConfig = hzConfig
		return keys
	}, 20*Second, interval).Should(ConsistOf(caches))
	return returnConfig
}

func assertMapConfigsPersisted(hazelcast *hazelcastv1beta1.Hazelcast, maps ...string) *config.HazelcastWrapper {
	cm := &corev1.Secret{}
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
		err = yaml.Unmarshal(cm.Data["hazelcast.yaml"], hzConfig)
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

func skipCleanup() bool {
	if CurrentSpecReport().State == ginkgoTypes.SpecStateSkipped {
		return true
	}
	if CurrentSpecReport().State != ginkgoTypes.SpecStatePassed {
		printDebugState()
	}
	return false
}

func printDebugState() {
	GinkgoWriter.Printf("Started aftereach function for hzLookupkey : '%s'\n", hzLookupKey)

	printed := false
	for _, context := range []string{context1, context2} {
		if context == "" {
			continue
		}
		printed = true
		SwitchContext(context)
		setupEnv()
		printDebugStateForContext()
	}
	if printed {
		GinkgoWriter.Printf("Current Ginkgo Spec Report State is: %+v\n", CurrentSpecReport().State)
		return
	}
	// print in the default context
	printDebugStateForContext()
	GinkgoWriter.Printf("Current Ginkgo Spec Report State is: %+v\n", CurrentSpecReport().State)
}

func printDebugStateForContext() {
	allCRs := "hazelcast,map,hotbackup,wanreplication,managementcenter,pvc,topic,queue,cache,multimap,replicatedmap,jetjob"
	printKubectlCommand("KUBECTL GET CLUSTER WIDE RESOURCES", "kubectl", "get", "node,validatingwebhookconfigurations")
	printKubectlCommand("KUBECTL GET CRS RELATED TO TEST OUTPUT WIDE", "kubectl", "get", allCRs, "-o=wide", "-l="+labelsString(), "-A")
	for _, ns := range []string{hzNamespace, sourceNamespace, targetNamespace} {
		if ns != "" {
			printKubectlCommand("KUBECTL GET ALL IN NAMESPACE "+ns, "kubectl", "get", "all", "-o=wide", "-n="+ns)
		}
	}
	printKubectlCommand("KUBECTL GET CRS RELATED TO TEST OUTPUT YAML", "kubectl", "get", allCRs, "-o=yaml", "-l="+labelsString(), "-A")

}

func printKubectlCommand(title, cmd string, args ...string) {
	GinkgoWriter.Println("## " + title)
	cmdCRs := exec.Command(cmd, args...)
	bytCRs, err := cmdCRs.Output()
	Expect(err).To(BeNil())
	GinkgoWriter.Println(" ", strings.ReplaceAll(string(bytCRs), "\n", "\n  "))
}

func labelsString() string {
	list := make([]string, 0, len(labels))
	for key, val := range labels {
		list = append(list, key+"="+val)
	}
	return strings.Join(list, ",")
}

func getExecutorServiceConfigFromMemberConfig(memberConfigXML string) codecTypes.ExecutorServices {
	var executorServices codecTypes.ExecutorServices
	err := xml.Unmarshal([]byte(memberConfigXML), &executorServices)
	Expect(err).To(BeNil())
	return executorServices
}

func assertExecutorServices(expectedES map[string]interface{}, actualES codecTypes.ExecutorServices) {
	for i, bes1 := range expectedES["es"].([]hazelcastcomv1alpha1.ExecutorServiceConfiguration) {
		// `i+1`'s reason is the default executor service added by hazelcast in any case.
		assertES(bes1, actualES.Basic[i+1])
	}
	for i, des1 := range expectedES["des"].([]hazelcastcomv1alpha1.DurableExecutorServiceConfiguration) {
		assertDurableES(des1, actualES.Durable[i])
	}
	for i, ses1 := range expectedES["ses"].([]hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration) {
		assertScheduledES(ses1, actualES.Scheduled[i])
	}
}

func assertES(expectedES hazelcastcomv1alpha1.ExecutorServiceConfiguration, actualES codecTypes.ExecutorServiceConfig) {
	Expect(expectedES.Name).Should(Equal(actualES.Name), "Name")
	Expect(expectedES.PoolSize).Should(Equal(actualES.PoolSize), "PoolSize")
	Expect(expectedES.QueueCapacity).Should(Equal(actualES.QueueCapacity), "QueueCapacity")
}

func assertDurableES(expectedDES hazelcastcomv1alpha1.DurableExecutorServiceConfiguration, actualDES codecTypes.DurableExecutorServiceConfig) {
	Expect(expectedDES.Name).Should(Equal(actualDES.Name), "Name")
	Expect(expectedDES.PoolSize).Should(Equal(actualDES.PoolSize), "PoolSize")
	Expect(expectedDES.Capacity).Should(Equal(actualDES.Capacity), "Capacity")
	Expect(expectedDES.Durability).Should(Equal(actualDES.Durability), "Durability")
}

func assertScheduledES(expectedSES hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration, actualSES codecTypes.ScheduledExecutorServiceConfig) {
	Expect(expectedSES.Name).Should(Equal(actualSES.Name), "Name")
	Expect(expectedSES.PoolSize).Should(Equal(actualSES.PoolSize), "PoolSize")
	Expect(expectedSES.Capacity).Should(Equal(actualSES.Capacity), "Capacity")
	Expect(expectedSES.Durability).Should(Equal(actualSES.Durability), "Durability")
	Expect(expectedSES.CapacityPolicy).Should(Equal(actualSES.CapacityPolicy), "CapacityPolicy")
}

func assertHotBackupStatus(hb *hazelcastcomv1alpha1.HotBackup, s hazelcastcomv1alpha1.HotBackupState, t Duration) *hazelcastcomv1alpha1.HotBackup {
	hbCheck := &hazelcastcomv1alpha1.HotBackup{}
	By(fmt.Sprintf("waiting for HotBackup CR status to be %s", s), func() {
		Eventually(func() hazelcastcomv1alpha1.HotBackupState {
			err := k8sClient.Get(
				context.Background(), types.NamespacedName{Name: hb.Name, Namespace: hzNamespace}, hbCheck)
			Expect(err).ToNot(HaveOccurred())
			Expect(hbCheck.Status.State).ShouldNot(Equal(hazelcastcomv1alpha1.HotBackupFailure), "Message: %v", hbCheck.Status.Message)
			return hbCheck.Status.State
		}, t, interval).Should(Equal(s))
	})
	return hbCheck
}

func assertHotBackupSuccess(hb *hazelcastcomv1alpha1.HotBackup, t Duration) *hazelcastcomv1alpha1.HotBackup {
	return assertHotBackupStatus(hb, hazelcastcomv1alpha1.HotBackupSuccess, t)
}

func assertDataStructureStatus(lk types.NamespacedName, st hazelcastcomv1alpha1.DataStructureConfigState, obj client.Object) client.Object {
	temp := fmt.Sprintf("waiting for %v CR status", hazelcastcomv1alpha1.GetKind(obj))
	By(temp, func() {
		Eventually(func() hazelcastcomv1alpha1.DataStructureConfigState {
			err := k8sClient.Get(context.Background(), lk, obj)
			if err != nil {
				return ""
			}
			return obj.(hazelcastcomv1alpha1.DataStructure).GetStatus().State
		}, 1*Minute, interval).Should(Equal(st))
	})
	return obj
}

func assertObjectDoesNotExist(obj client.Object) {
	cpy := obj.DeepCopyObject().(client.Object)
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, cpy)
		if err == nil || !kerrors.IsNotFound(err) {
			return false
		}
		return true
	}, 1*Minute, interval).Should(BeTrue())
}

func getMultiMapConfigFromMemberConfig(memberConfigXML string, multiMapName string) *codecTypes.MultiMapConfig {
	var multiMaps codecTypes.MultiMapConfigs
	err := xml.Unmarshal([]byte(memberConfigXML), &multiMaps)
	Expect(err).To(BeNil())
	for _, mm := range multiMaps.MultiMaps {
		if mm.Name == multiMapName {
			return &mm
		}
	}
	return nil
}

func getTopicConfigFromMemberConfig(memberConfigXML string, topicName string) *codecTypes.TopicConfig {
	var topics codecTypes.TopicConfigs
	err := xml.Unmarshal([]byte(memberConfigXML), &topics)
	Expect(err).To(BeNil())
	for _, t := range topics.Topics {
		if t.Name == topicName {
			return &t
		}
	}
	return nil
}

func getReplicatedMapConfigFromMemberConfig(memberConfigXML string, replicatedMapName string) *codecTypes.ReplicatedMapConfig {
	var replicatedMaps codecTypes.ReplicatedMapConfigs
	err := xml.Unmarshal([]byte(memberConfigXML), &replicatedMaps)
	Expect(err).To(BeNil())
	for _, rm := range replicatedMaps.ReplicatedMaps {
		if rm.Name == replicatedMapName {
			return &rm
		}
	}
	return nil
}

func getQueueConfigFromMemberConfig(memberConfigXML string, queueName string) *codecTypes.QueueConfigInput {
	var queues codecTypes.QueueConfigs
	err := xml.Unmarshal([]byte(memberConfigXML), &queues)
	Expect(err).To(BeNil())
	for _, q := range queues.Queues {
		if q.Name == queueName {
			return &q
		}
	}
	return nil
}

func DnsLookupAddressMatched(ctx context.Context, host, addr string) (bool, error) {
	IPs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return false, err
	}
	for _, IP := range IPs {
		if IP == addr {
			return true, nil
		}
	}
	return false, nil
}

func getCacheConfigFromMemberConfig(memberConfigXML string, cacheName string) *codecTypes.CacheConfigInput {
	var caches codecTypes.CacheConfigs
	err := xml.Unmarshal([]byte(memberConfigXML), &caches)
	Expect(err).To(BeNil())
	for _, c := range caches.Caches {
		if c.Name == cacheName {
			return &c
		}
	}
	return nil
}

func unixMilli(msec int64) Time {
	return Unix(msec/1e3, (msec%1e3)*1e6)
}

func assertCorrectBackupStatus(hotBackup *hazelcastcomv1alpha1.HotBackup, seq string) {
	if hotBackup.Spec.IsExternal() {
		timestamp, _ := strconv.ParseInt(seq, 10, 64)
		bucketURI := hotBackup.Spec.BucketURI + fmt.Sprintf("?prefix=%s/%s/", hzLookupKey.Name,
			unixMilli(timestamp).UTC().Format("2006-01-02-15-04-05")) // hazelcast/2022-06-02-21-57-49/

		backupBucketURI := hotBackup.Status.GetBucketURI()
		Expect(bucketURI).Should(Equal(backupBucketURI))
		return
	}

	// if local backup
	backupSeqFolder := hotBackup.Status.GetBackupFolder()
	Expect("backup-" + seq).Should(Equal(backupSeqFolder))
}

func restoreConfig(hotBackup *hazelcastcomv1alpha1.HotBackup, useBucketConfig bool) hazelcastv1beta1.RestoreConfiguration {
	if useBucketConfig {
		return hazelcastv1beta1.RestoreConfiguration{
			BucketConfiguration: &hazelcastv1beta1.BucketConfiguration{
				BucketURI: hotBackup.Status.GetBucketURI(),
				Secret:    hotBackup.Spec.Secret,
			},
		}
	}
	return hazelcastv1beta1.RestoreConfiguration{
		HotBackupResourceName: hotBackup.Name,
	}
}

func createWanResources(ctx context.Context, hzMapResources map[string][]string, ns string, labels map[string]string) (map[string]*hazelcastv1beta1.Hazelcast, map[string]*hazelcastcomv1alpha1.Map) {
	hzCrs := map[string]*hazelcastv1beta1.Hazelcast{}

	for hzCrName := range hzMapResources {
		hz := hazelcastconfig.Default(types.NamespacedName{Name: hzCrName, Namespace: ns}, ee, labels)
		hz.Spec.ClusterName = hzCrName
		hz.Spec.ClusterSize = pointer.Int32(1)
		hzCrs[hzCrName] = hz
		CreateHazelcastCRWithoutCheck(hz)
	}

	for _, hz := range hzCrs {
		evaluateReadyMembers(types.NamespacedName{Name: hz.Name, Namespace: ns})
	}

	mapCrs := map[string]*hazelcastcomv1alpha1.Map{}

	for hzCrName, mapCrNames := range hzMapResources {
		for _, mapCrName := range mapCrNames {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapCrName, Namespace: ns}, hzCrName, labels)
			mapCrs[mapCrName] = m
			Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		}
	}

	for i := range mapCrs {
		mapCrs[i] = assertMapStatus(mapCrs[i], hazelcastcomv1alpha1.MapSuccess)
	}

	return hzCrs, mapCrs

}

func createWanConfig(ctx context.Context, lk types.NamespacedName, target *hazelcastv1beta1.Hazelcast, resources []hazelcastcomv1alpha1.ResourceSpec, mapCount int, labels map[string]string) *hazelcastcomv1alpha1.WanReplication {
	wan := hazelcastconfig.WanReplication(
		lk,
		target.Spec.ClusterName,
		fmt.Sprintf("%s.%s.svc.cluster.local:%d", target.Name, target.Namespace, naming.WanDefaultPort),
		resources,
		labels,
	)
	Expect(k8sClient.Create(ctx, wan)).Should(Succeed())
	wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
	wan = assertWanStatusMapCount(wan, mapCount)
	return wan
}

func CreateMcForClusters(ctx context.Context, hzCrs ...*hazelcastv1beta1.Hazelcast) {
	clusters := []hazelcastcomv1alpha1.HazelcastClusterConfig{}
	for _, hz := range hzCrs {
		clusters = append(clusters, hazelcastcomv1alpha1.HazelcastClusterConfig{Name: hz.Spec.ClusterName, Address: hzclient.HazelcastUrl(hz)})
	}
	mc := mcconfig.WithClusterConfig(mcLookupKey, ee, clusters, labels)
	Expect(k8sClient.Create(ctx, mc)).Should(Succeed())
}

func createMapCRWithMapName(ctx context.Context, mapCrName, mapName string, hzLookupKey types.NamespacedName) *hazelcastcomv1alpha1.Map {
	m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapCrName, Namespace: hzLookupKey.Namespace}, hzLookupKey.Name, labels)
	m.Spec.Name = mapName
	Expect(k8sClient.Create(ctx, m)).Should(Succeed())
	assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
	return m
}
