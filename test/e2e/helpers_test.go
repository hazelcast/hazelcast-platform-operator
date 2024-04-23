package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
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
	"sync"
	"time"
	. "time"

	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
	"github.com/hazelcast/hazelcast-platform-operator/test"

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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/naming"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/config"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"

	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
)

type UpdateFn func(*hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast
type PodLabel struct {
	Selector   string
	LabelKey   string
	LabelValue string
}

func InitLogs(t Time, lk types.NamespacedName) io.ReadCloser {
	var logs io.ReadCloser
	By("getting Hazelcast logs", func() {
		logs = GetPodLogs(context.Background(), types.NamespacedName{
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
		logs = GetPodLogs(context.Background(), types.NamespacedName{
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

func CreateHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Eventually(func() error {
			return k8sClient.Create(context.Background(), hazelcast)
		}, 10*Minute, interval).Should(Succeed())
	})
	lk := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
	message := ""
	By("checking Hazelcast CR running", func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
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

func UpdateHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast, fns ...UpdateFn) {
	By("updating the CR", func() {
		if len(fns) == 0 {
			Expect(k8sClient.Update(context.Background(), hazelcast)).Should(Succeed())
		} else {
			lk := types.NamespacedName{Name: hazelcast.Name, Namespace: hazelcast.Namespace}
			for {
				cr := &hazelcastcomv1alpha1.Hazelcast{}
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

func CreateHazelcastCRWithoutCheck(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("creating Hazelcast CR", func() {
		Eventually(func() error {
			return k8sClient.Create(context.Background(), hazelcast)
		}, 10*Minute, interval).Should(Succeed())
	})
}

func RemoveHazelcastCR(hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	By("removing hazelcast CR", func() {
		Eventually(func() error {
			return k8sClient.Delete(context.Background(), hazelcast, client.PropagationPolicy(metav1.DeletePropagationForeground))
		}, Minute, interval).Should(Succeed())
		assertDoesNotExist(types.NamespacedName{
			Name:      hazelcast.Name + "-0",
			Namespace: hazelcast.Namespace,
		}, &corev1.Pod{})
	})
	By("waiting for Hazelcast CR to be removed", func() {
		Eventually(func() error {
			h := &hazelcastcomv1alpha1.Hazelcast{}
			return k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      hazelcast.Name,
				Namespace: hazelcast.Namespace,
			}, h)
		}, 5*Minute, interval).ShouldNot(Succeed())
	})
}

func DeletePod(podName string, gracePeriod int64, lk types.NamespacedName) {
	By(fmt.Sprintf("deleting POD with name '%s'", podName), func() {
		propagationPolicy := metav1.DeletePropagationForeground
		deleteOptions := metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			PropagationPolicy:  &propagationPolicy,
		}
		podExists := func() bool {
			_, err := getKubernetesClientSet().CoreV1().Pods(lk.Namespace).Get(context.Background(), podName, metav1.GetOptions{})
			return err != nil && !errors.IsNotFound(err)
		}
		err := getKubernetesClientSet().CoreV1().Pods(lk.Namespace).Delete(context.Background(), podName, deleteOptions)
		if err != nil && !errors.IsNotFound(err) {
			log.Fatal(err)
		}
		attempt := 0
		maxAttempts := 5
		var waitTime time.Duration = 2
		for attempt < maxAttempts && podExists() {
			log.Printf("Pod '%s' still exists. Waiting %d seconds before retrying delete operation.\n", podName, waitTime)
			time.Sleep(waitTime * time.Second)
			err := getKubernetesClientSet().CoreV1().Pods(lk.Namespace).Delete(context.Background(), podName, deleteOptions)
			if err != nil && !errors.IsNotFound(err) {
				log.Fatal(err)
			}
			attempt++
			waitTime *= 2
		}
	})
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

func getKubernetesClientSet() *kubernetes.Clientset {
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

func FillMapByEntryCount(ctx context.Context, lk types.NamespacedName, unisocket bool, mapName string, entryCount int) error {
	_ = fmt.Sprintf("filling the '%s' map with '%d' entries using '%s' lookup name and '%s' namespace", mapName, entryCount, lk.Name, lk.Namespace)
	var m *hzClient.Map
	clientHz := GetHzClient(ctx, lk, unisocket)
	m, err := clientHz.GetMap(ctx, mapName)
	if err != nil {
		return fmt.Errorf("error getting map '%s': %w", mapName, err)
	}
	initMapSize, err := m.Size(ctx)
	if err != nil {
		return fmt.Errorf("error getting initial size of map '%s': %w", mapName, err)
	}
	entries := make([]hzclienttypes.Entry, 0, entryCount)
	for i := initMapSize; i < initMapSize+entryCount; i++ {
		entries = append(entries, hzclienttypes.NewEntry(strconv.Itoa(i), strconv.Itoa(i)))
	}
	err = m.PutAll(ctx, entries...)
	if err != nil {
		return fmt.Errorf("error putting entries into map '%s': %w", mapName, err)
	}
	mapSize, err := m.Size(ctx)
	if err != nil {
		return fmt.Errorf("error getting updated size of map '%s': %w", mapName, err)
	}
	Expect(mapSize).To(Equal(initMapSize + entryCount))
	err = clientHz.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error shutting down Hazelcast client: %w", err)
	}
	return nil
}

/*
2 (entries per single goroutine) = 1048576 (Bytes per 1Mb)  / 8192 (Bytes per entry) / 64 (goroutines)
*/

func FillMapBySizeInMb(ctx context.Context, mapName string, sizeInMb int, expectedSize int, hzConfig *hazelcastcomv1alpha1.Hazelcast) {
	fmt.Printf("filling the map '%s' with '%d' MB data\n", mapName, sizeInMb)
	hzAddress := hzclient.HazelcastUrl(hzConfig)
	clientHz := GetHzClient(ctx, types.NamespacedName{Name: hzConfig.Name, Namespace: hzConfig.Namespace}, true)
	defer func() {
		err := clientHz.Shutdown(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
	mapLoaderPod := createMapLoaderPod(hzAddress, hzConfig.Spec.ClusterName, sizeInMb, mapName, types.NamespacedName{Name: hzConfig.Name, Namespace: hzConfig.Namespace})
	defer DeletePod(mapLoaderPod.Name, 10, types.NamespacedName{Namespace: hzConfig.Namespace})
	fmt.Printf("Expected entries for the map '%s' is %d\n", mapName, expectedSize*128)
	Eventually(func() int {
		return countKeySet(ctx, clientHz, mapName, hzConfig)
	}, 15*Minute, interval).Should(Equal(expectedSize * 128)) // 128 entries/Mb = 2 (entries) * 64 (goroutines)
}

func countKeySet(ctx context.Context, clientHz *hzClient.Client, mapName string, hzConfig *hazelcastcomv1alpha1.Hazelcast) int {
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

func ConcurrentlyCreateMaps(ctx context.Context, numMaps int, mapNameSuffix string, hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	errCh := make(chan error, numMaps)

	for i := 0; i < numMaps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, mapNameSuffix), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			m.Spec.PersistenceEnabled = true
			if err := k8sClient.Create(ctx, m); err != nil {
				errCh <- err
				return
			}
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		}(i)
	}
	wg.Wait()
	close(errCh)
}

func ConcurrentlyFillMultipleMapsByMb(ctx context.Context, numMaps int, sizePerMap int, mapNameSuffix string, expectedSize int, hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for i := 0; i < numMaps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			FillMapBySizeInMb(ctx, fmt.Sprintf("map-%d-%s", i, mapNameSuffix), sizePerMap, expectedSize, hazelcast)
		}(i)
	}
	wg.Wait()
}

func ConcurrentlyCreateAndFillMultipleMapsByMb(numMaps int, sizePerMap int, mapNameSuffix string, hazelcast *hazelcastcomv1alpha1.Hazelcast) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	createErrCh := make(chan error, numMaps)
	fillErrCh := make(chan error, numMaps)
	mapReadyCh := make(chan *hazelcastcomv1alpha1.Map, numMaps)
	for i := 0; i < numMaps; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, mapNameSuffix), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			m.Spec.PersistenceEnabled = true
			if err := k8sClient.Create(context.Background(), m); err != nil {
				createErrCh <- err
				return
			}
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
			mapReadyCh <- m // Signal that the map is ready to be filled
		}(i)
	}

	go func() {
		for m := range mapReadyCh {
			wg.Add(1)
			go func(m *hazelcastcomv1alpha1.Map) {
				defer wg.Done()
				FillMapBySizeInMb(context.Background(), m.MapName(), sizePerMap, sizePerMap, hazelcast)
			}(m)
		}
	}()
	wg.Wait()
	close(createErrCh)
	close(fillErrCh)
	close(mapReadyCh)
	for err := range createErrCh {
		log.Printf("Error creating map: %v", err)
	}
	for err := range fillErrCh {
		log.Printf("Error filling map: %v", err)
	}
}

func WaitForMapSize(ctx context.Context, lk types.NamespacedName, mapName string, expectedMapSize int, timeout time.Duration) {
	fmt.Printf("Waiting for the '%s' map to be of size '%d' using lookup name '%s'\n", mapName, expectedMapSize, lk.Name)
	if timeout == 0 {
		timeout = 15 * time.Minute
		log.Printf("No timeout specified, defaulting to %v\n", timeout)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	clientHz := GetHzClient(ctxWithTimeout, lk, true)

	defer func() {
		log.Println("Shutting down Hazelcast client")
		if err := clientHz.Shutdown(ctxWithTimeout); err != nil {
			log.Printf("Error while shutting down Hazelcast client: %v\n", err)
			Expect(err).ToNot(HaveOccurred())
		}
	}()

	hzMap, err := clientHz.GetMap(ctxWithTimeout, mapName)
	if err != nil {
		log.Printf("Failed to get map '%s': %v\n", mapName, err)
		Expect(err).ToNot(HaveOccurred())
	}

	Eventually(func() (int, error) {
		mapSize, err := hzMap.Size(ctxWithTimeout)
		if err != nil {
			log.Printf("Error getting size of map '%s': %v\n", mapName, err)
			return 0, err
		}
		log.Printf("Current size of map '%s': %d\n", mapName, mapSize)
		return mapSize, nil
	}, timeout, time.Minute).Should(Equal(expectedMapSize))

	log.Printf("Map '%s' reached expected size '%d'\n", mapName, expectedMapSize)
}

func LabelPods(namespace string, podLabels []PodLabel) error {
	clientset := getKubernetesClientSet()
	for _, pl := range podLabels {
		pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: pl.Selector})
		if err != nil {
			return err
		}

		for _, pod := range pods.Items {
			if pod.Labels == nil {
				pod.Labels = make(map[string]string)
			}
			pod.Labels[pl.LabelKey] = pl.LabelValue
			_, err := clientset.CoreV1().Pods(namespace).Update(context.Background(), &pod, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func CountRunningPods(pods []corev1.Pod) int {
	runningPods := 0
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}
	return runningPods
}

func DeleteConfigMap(namespace, name string) {
	deletePolicy := metav1.DeletePropagationForeground
	err := getKubernetesClientSet().CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		log.Printf("Failed to delete ConfigMap %s in namespace %s: %v", name, namespace, err)
	} else {
		log.Printf("ConfigMap %s in namespace %s deleted successfully", name, namespace)
	}
}

func WaitForPodReady(podName string, lk types.NamespacedName, timeout time.Duration) {
	watcher, err := getKubernetesClientSet().CoreV1().Pods(lk.Namespace).Watch(context.Background(), metav1.ListOptions{FieldSelector: "metadata.name=" + podName})
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

func createMapLoaderPod(hzAddress, clusterName string, mapSizeInMb int, mapName string, lk types.NamespacedName) *corev1.Pod {
	name := randString(5)
	size := strconv.Itoa(mapSizeInMb)
	clientPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"maploader": "true",
			},
			Name:      "maploader-" + name,
			Namespace: lk.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "maploader-container",
					Image: getMapLoaderImage(),
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
	_, err := getKubernetesClientSet().CoreV1().Pods(lk.Namespace).Create(context.Background(), clientPod, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      clientPod.Name,
		Namespace: lk.Namespace,
	}, clientPod)
	Expect(err).ToNot(HaveOccurred())
	return clientPod
}

func getMapLoaderImage() string {
	img, ok := os.LookupEnv("MAPLOADER_IMAGE")
	if ok {
		return img
	}
	return "us-east1-docker.pkg.dev/hazelcast-33/hazelcast-platform-operator/wan-replication-maploader:700d"
}

func isHazelcastRunning(hz *hazelcastcomv1alpha1.Hazelcast) bool {
	return hz.Status.Phase == hazelcastcomv1alpha1.Running
}

func isManagementCenterRunning(mc *hazelcastcomv1alpha1.ManagementCenter) bool {
	return mc.Status.Phase == hazelcastcomv1alpha1.McRunning
}

func evaluateReadyMembers(lookupKey types.NamespacedName) {
	By(fmt.Sprintf("evaluate number of ready members for lookup name '%s' and '%s' namespace", lookupKey.Name, lookupKey.Namespace), func() {
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() error {
			err := k8sClient.Get(context.Background(), lookupKey, hz)
			if err != nil {
				fmt.Printf("Error occurred while getting Hazelcast resource: %s\n", err)
				return err
			}
			return nil
		}, 5*Minute, interval).Should(BeNil())
		membersCount := int(*hz.Spec.ClusterSize)
		Eventually(func() (string, error) {
			err := k8sClient.Get(context.Background(), lookupKey, hz)
			if err != nil {
				fmt.Printf("Error occurred while getting Hazelcast ready members: %s\n", err)
				return "", err
			}
			return hz.Status.Cluster.ReadyMembers, nil
		}, 15*Minute, interval).Should(Equal(fmt.Sprintf("%d/%d", membersCount, membersCount)))
	})
}

func WaitForReplicaSize(namespace string, resourceName string, replicas int32) {
	clientSet := getKubernetesClientSet()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	Eventually(func() (int32, error) {
		sts, err := clientSet.AppsV1().StatefulSets(namespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		return sts.Status.CurrentReplicas, nil
	}, 10*Minute, interval).Should(Equal(replicas))
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

func newHazelcastClientPortForward(ctx context.Context, h *hazelcastcomv1alpha1.Hazelcast, localPort string) *hzClient.Client {
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

func assertMapConfigsPersisted(hazelcast *hazelcastcomv1alpha1.Hazelcast, maps ...string) *config.HazelcastWrapper {
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

func assertUserCodeNamespaceStatus(m *hazelcastcomv1alpha1.UserCodeNamespace, st hazelcastcomv1alpha1.UserCodeNamespaceState) *hazelcastcomv1alpha1.UserCodeNamespace {
	checkUCN := &hazelcastcomv1alpha1.UserCodeNamespace{}
	By("waiting for UserCodeNamespace CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.UserCodeNamespaceState {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      m.Name,
				Namespace: m.Namespace,
			}, checkUCN)
			if err != nil {
				return ""
			}
			return checkUCN.Status.State
		}, 40*Second, interval).Should(Equal(st))
	})
	return checkUCN
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

func assertWanSyncStatus(wr *hazelcastcomv1alpha1.WanSync, st hazelcastcomv1alpha1.WanSyncPhase) *hazelcastcomv1alpha1.WanSync {
	checkWan := &hazelcastcomv1alpha1.WanSync{}
	By("waiting for WAN CR status", func() {
		Eventually(func() hazelcastcomv1alpha1.WanSyncPhase {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      wr.Name,
				Namespace: wr.Namespace,
			}, checkWan)
			if err != nil {
				return ""
			}
			Expect(checkWan.Status.Status).ShouldNot(Equal(hazelcastcomv1alpha1.WanSyncFailed))
			return checkWan.Status.Status
		}, 5*Minute, interval).Should(Equal(st))
	})
	return checkWan
}

func assertWanSyncStatusMapCount(wr *hazelcastcomv1alpha1.WanSync, mapLen int) *hazelcastcomv1alpha1.WanSync {
	checkWan := &hazelcastcomv1alpha1.WanSync{}
	By("waiting for WAN CR status map length", func() {
		Eventually(func() int {
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      wr.Name,
				Namespace: wr.Namespace,
			}, checkWan)
			if err != nil {
				return -1
			}
			return len(checkWan.Status.WanSyncMapsStatus)
		}, 1*Minute, interval).Should(Equal(mapLen))
	})
	return checkWan
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
			if checkHz.Status.Restore == (hazelcastcomv1alpha1.RestoreStatus{}) {
				return ""
			}
			return checkHz.Status.Restore.State
		}, 40*Second, interval).Should(Equal(st))
	})
	return checkHz
}

func assertCacheConfigsPersisted(hazelcast *hazelcastcomv1alpha1.Hazelcast, caches ...string) *config.HazelcastWrapper {
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

// assertMemberLogs check that the given expected string can be found in the logs.
// expected can be a regexp pattern.
func assertMemberLogs(h *hazelcastcomv1alpha1.Hazelcast, expected string) {
	logs := GetPodLogs(context.Background(), types.NamespacedName{
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

func assertMembersNotRestarted(lookupKey types.NamespacedName) {
	hz := &hazelcastcomv1alpha1.Hazelcast{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), lookupKey, hz)
	}, Minute, interval).ShouldNot(HaveOccurred())
	membersCount := int(*hz.Spec.ClusterSize)
	for i := 0; i < membersCount; i++ {
		podLookupKey := types.NamespacedName{
			Name:      fmt.Sprintf("%s-%d", lookupKey.Name, i),
			Namespace: lookupKey.Namespace,
		}
		pod := corev1.Pod{}
		Eventually(func() error {
			return k8sClient.Get(context.Background(), podLookupKey, &pod)
		}, Minute, interval).ShouldNot(HaveOccurred())
		Expect(pod.Status.ContainerStatuses[0].RestartCount).To(BeZero())
	}
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

func getExecutorServiceConfigFromMemberConfig(memberConfigXML string) codecTypes.ExecutorServices {
	var executorServices codecTypes.ExecutorServices
	err := xml.Unmarshal([]byte(memberConfigXML), &executorServices)
	Expect(err).To(BeNil())
	return executorServices
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

func getMapConfigFromMemberConfig(memberConfigXML string, mapName string) *codecTypes.AddMapConfigInput {
	var maps codecTypes.MapConfigs
	err := xml.Unmarshal([]byte(memberConfigXML), &maps)
	Expect(err).To(BeNil())
	for _, m := range maps.Maps {
		if m.Name == mapName {
			return &m
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

func restoreConfig(hotBackup *hazelcastcomv1alpha1.HotBackup, useBucketConfig bool) hazelcastcomv1alpha1.RestoreConfiguration {
	if useBucketConfig {
		return hazelcastcomv1alpha1.RestoreConfiguration{
			BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
				BucketURI:  hotBackup.Status.GetBucketURI(),
				SecretName: hotBackup.Spec.GetSecretName(),
			},
		}
	}
	return hazelcastcomv1alpha1.RestoreConfiguration{
		HotBackupResourceName: hotBackup.Name,
	}
}

func createWanResources(ctx context.Context, hzMapResources map[string][]string, ns string, labels map[string]string, mtFn ...func(p *hazelcastcomv1alpha1.Map)) (map[string]*hazelcastcomv1alpha1.Hazelcast, map[string]*hazelcastcomv1alpha1.Map) {
	hzCrs := map[string]*hazelcastcomv1alpha1.Hazelcast{}
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
			for _, f := range mtFn {
				f(m)
			}
			mapCrs[mapCrName] = m
			Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		}
	}
	for i := range mapCrs {
		mapCrs[i] = assertMapStatus(mapCrs[i], hazelcastcomv1alpha1.MapSuccess)
	}
	return hzCrs, mapCrs
}

func createWanConfig(ctx context.Context, lk types.NamespacedName, target *hazelcastcomv1alpha1.Hazelcast, resources []hazelcastcomv1alpha1.ResourceSpec, mapCount int, labels map[string]string, mtFn ...func(*hazelcastcomv1alpha1.WanReplication)) *hazelcastcomv1alpha1.WanReplication {
	wan := hazelcastconfig.WanReplication(
		lk,
		target.Spec.ClusterName,
		fmt.Sprintf("%s.%s.svc.cluster.local:%d", target.Name, target.Namespace, naming.WanDefaultPort),
		resources,
		labels,
	)
	for _, f := range mtFn {
		f(wan)
	}
	Expect(k8sClient.Create(ctx, wan)).Should(Succeed())
	wan = assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)
	wan = assertWanStatusMapCount(wan, mapCount)
	return wan
}

func createWanSync(ctx context.Context, lk types.NamespacedName, wanReplicationName string, mapCount int, labels map[string]string) *hazelcastcomv1alpha1.WanSync {
	wan := hazelcastconfig.WanSync(lk, wanReplicationName, labels)
	Expect(k8sClient.Create(ctx, wan)).Should(Succeed())
	wan = assertWanSyncStatus(wan, hazelcastcomv1alpha1.WanSyncCompleted)
	wan = assertWanSyncStatusMapCount(wan, mapCount)
	return wan
}

func CreateMcForClusters(ctx context.Context, hzCrs ...*hazelcastcomv1alpha1.Hazelcast) {
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

func ScaleStatefulSet(namespace string, resourceName string, replicas int32) {
	clientSet := getKubernetesClientSet()
	sts, _ := clientSet.AppsV1().StatefulSets(namespace).GetScale(context.TODO(), resourceName, metav1.GetOptions{})
	sts.Spec.Replicas = replicas
	_, err := clientSet.AppsV1().StatefulSets(namespace).UpdateScale(context.TODO(), resourceName, sts, metav1.UpdateOptions{})
	if err != nil {
		return
	}
	Sleep(10 * Second)
	WaitForReplicaSize(namespace, resourceName, replicas)
}

func RolloutRestart(ctx context.Context, hazelcast *hazelcastcomv1alpha1.Hazelcast) error {
	clientSet := getKubernetesClientSet()
	statefulSets, err := clientSet.AppsV1().StatefulSets(hazelcast.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list stateful sets: %w", err)
	}
	if len(statefulSets.Items) == 0 {
		return fmt.Errorf("no stateful sets found")
	}

	sts, err := clientSet.AppsV1().StatefulSets(hazelcast.Namespace).Get(ctx, statefulSets.Items[0].Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get stateful set: %w", err)
	}

	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations["kubectl-rollout-restart"] = time.Now().Format(time.RFC3339)

	_, err = clientSet.AppsV1().StatefulSets(hazelcast.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update stateful set: %w", err)
	}
	Sleep(10 * time.Second)
	return nil
}

func fetchHazelcastEndpoints(hz *hazelcastcomv1alpha1.Hazelcast) []hazelcastcomv1alpha1.HazelcastEndpoint {
	hzEndpointList := hazelcastcomv1alpha1.HazelcastEndpointList{}
	err := k8sClient.List(context.Background(), &hzEndpointList,
		client.InNamespace(hz.Namespace),
		client.MatchingLabels(util.Labels(hz)),
	)
	Expect(err).ToNot(HaveOccurred())
	return hzEndpointList.Items
}

func fetchHazelcastAddressesByType(hz *hazelcastcomv1alpha1.Hazelcast, endpointType ...hazelcastcomv1alpha1.HazelcastEndpointType) []string {
	hzEndpoints := fetchHazelcastEndpoints(hz)
	hazelcastEndpointTypeMap := make(map[hazelcastcomv1alpha1.HazelcastEndpointType]struct{}, len(endpointType))
	for _, hazelcastEndpointType := range endpointType {
		hazelcastEndpointTypeMap[hazelcastEndpointType] = struct{}{}
	}
	var addresses []string
	for _, hzEndpoint := range hzEndpoints {
		endpointNn := types.NamespacedName{Name: hzEndpoint.Name, Namespace: hzEndpoint.Namespace}
		_, ok := hazelcastEndpointTypeMap[hzEndpoint.Spec.Type]
		if ok {
			Eventually(func() string {
				Expect(k8sClient.Get(context.Background(), endpointNn, &hzEndpoint)).ToNot(HaveOccurred())
				return hzEndpoint.Status.Address
			}, 3*time.Minute, interval).Should(Not(BeEmpty()))
			Expect(k8sClient.Get(context.Background(), endpointNn, &hzEndpoint)).ToNot(HaveOccurred())
			addresses = append(addresses, hzEndpoint.Status.Address)
		}
	}
	return addresses
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

	GinkgoWriter.Println("## Manifests")
	GinkgoWriter.Print(recordedManifests[hzLookupKey])
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

func GetPodLogs(ctx context.Context, pod types.NamespacedName, podLogOptions *corev1.PodLogOptions) io.ReadCloser {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	p, err := clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
	if err != nil {
		panic(err)
	}
	if p.Status.Phase != corev1.PodFailed && p.Status.Phase != corev1.PodRunning {
		panic("Unable to get pod logs for the pod in Phase " + p.Status.Phase)
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, podLogOptions)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		panic(err)
	}
	return podLogs
}

func CheckPodStatus(pod types.NamespacedName, status corev1.PodPhase) {
	clientSet := getKubernetesClientSet()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	Eventually(func() corev1.PodPhase {
		pod, err := clientSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			panic(err)
		}
		return pod.Status.Phase
	}, 10*Minute, interval).Should(Equal(status))
}
