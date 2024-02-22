package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"
	. "time"

	chaosmeshv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cli "sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Platform Resilience Tests", Group("resilience"), func() {
	ctx := context.Background()

	BeforeEach(func() {
		By("checking chaos-mesh-operator running", func() {
			var podList corev1.PodList
			err := k8sClient.List(ctx, &podList, client.InNamespace(chaosMeshNamespace))
			Expect(err).To(BeNil())
			podNames := make([]string, 0, len(podList.Items))
			for _, pod := range podList.Items {
				podNames = append(podNames, pod.Name)
			}
			Expect(err).To(BeNil())
			Expect(podNames).Should(ContainElement(ContainSubstring("chaos-controller-manager")))
			Expect(podNames).Should(ContainElement(ContainSubstring("chaos-daemon")))
		})

		By("checking Kubernetes topology is proper for running the tests", func() {
			zoneNodeNameMap, err := nodeNamesInZones(context.Background())
			Expect(err).To(BeNil())
			Expect(len(zoneNodeNameMap)).Should(BeNumerically(">", 1))
			for _, nodes := range zoneNodeNameMap {
				Expect(len(nodes)).Should(BeNumerically(">", 1))
			}
		})

		By("checking all nodes are ready", func() {
			numberOfNodes, err := numberOfAllNodes(ctx)
			Expect(err).To(BeNil())
			readyNodesNumber, err := numberOfReadyNodes(ctx)
			Expect(err).To(BeNil())
			Expect(readyNodesNumber).To(Equal(numberOfNodes))
		})
	})

	AfterEach(func() {
		GinkgoWriter.Printf("AfterEach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		DeleteAllOf(&hazelcastcomv1alpha1.ManagementCenter{}, nil, hzNamespace, labels)
		deletePVCs(mcLookupKey)
		DeleteConfigMap(hzNamespace, "split-brain-config")
		By("waiting for all nodes are ready", func() {
			waitForDroppedNodes(context.Background(), 0)
		})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should kill the pod randomly and preserve the data after restore", Tag(Any), Serial, func() {
		setLabelAndCRName("hr-3")
		duration := "30s"
		mapSizeInMb := 500
		nMaps := 5
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
		restConfig, _ := kubeConfig.ClientConfig()
		k8sClient, _ := cli.New(restConfig, cli.Options{Scheme: scheme.Scheme})

		By("creating Hazelcast cluster with partition count and 10 maps")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, 3, labels)
		hazelcast.Name = hzLookupKey.Name
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(5) + "Gi")}[0]
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("label pods")
		podLabels := []PodLabel{
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-0", "group", "cm_pod_kill"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-1", "group", "cm_pod_kill"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-2", "group", "cm_pod_kill"},
		}
		err := LabelPods(hazelcast.Namespace, podLabels)
		if err != nil {
			fmt.Printf("Error labeling pods: %s\n", err)
		} else {
			fmt.Println("Pods labeled successfully")
		}
		By("create 5 maps")
		ConcurrentlyCreateMaps(context.Background(), nMaps, hazelcast.Name, hazelcast)

		By("put the entries")
		ConcurrentlyFillMultipleMapsByMb(context.Background(), nMaps, mapSizeInMb, hazelcast.Name, mapSizeInMb, hazelcast)

		By("run chaos mesh pod kill scenario")
		podKillChaos := &chaosmeshv1alpha1.PodChaos{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hazelcast.Name,
				Namespace: hzNamespace,
			},
			Spec: chaosmeshv1alpha1.PodChaosSpec{
				Action:   chaosmeshv1alpha1.PodKillAction,
				Duration: &duration,
				ContainerSelector: chaosmeshv1alpha1.ContainerSelector{
					PodSelector: chaosmeshv1alpha1.PodSelector{
						Selector: chaosmeshv1alpha1.PodSelectorSpec{
							GenericSelectorSpec: chaosmeshv1alpha1.GenericSelectorSpec{
								LabelSelectors: map[string]string{
									"group": "cm_pod_kill",
								},
							},
						},
						Mode: chaosmeshv1alpha1.OneMode,
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), podKillChaos)).Error().NotTo(HaveOccurred())

		By("checking the member size after random pod kill")
		Eventually(func() int {
			newPods, err := getKubernetesClientSet().CoreV1().Pods(hazelcast.Namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: "group=cm_pod_kill",
			})
			Expect(err).NotTo(HaveOccurred())
			return CountRunningPods(newPods.Items)
		}, 1*Minute, 1*Second).Should(Equal(2), "There should be exactly 2 pods in 'Running' status")

		By("checking map size after kill and resume")
		for i := 0; i < nMaps; i++ {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, hazelcast.Name), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			WaitForMapSize(context.Background(), hzLookupKey, m.MapName(), int(float64(mapSizeInMb*128)), 1*Minute)
		}
	})

	It("should check a split-brain protection in the Hazelcast cluster", Tag(Any), Serial, func() {
		setLabelAndCRName("hr-4")
		duration := "100s"
		splitBrainConfName := "splitBrainProtectionRuleWithFourMembers"
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
		restConfig, _ := kubeConfig.ClientConfig()
		k8sClient, _ := cli.New(restConfig, cli.Options{Scheme: scheme.Scheme})

		By("creating custom config with split-brain protection")
		customConfig := make(map[string]interface{})
		sbConf := make(map[string]interface{})
		sbConf[splitBrainConfName] = map[string]interface{}{
			"enabled":              true,
			"minimum-cluster-size": 4,
		}
		customConfig["split-brain-protection"] = sbConf

		mapConf := make(map[string]interface{})
		mapConf[mapLookupKey.Name] = map[string]interface{}{
			"split-brain-protection-ref": "splitBrainProtectionRuleWithFourMembers",
		}
		customConfig["map"] = mapConf

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "split-brain-config",
				Namespace: hzNamespace,
				Labels:    labels,
			},
		}
		out, err := yaml.Marshal(customConfig)
		Expect(err).To(BeNil())
		cm.Data = make(map[string]string)
		cm.Data["hazelcast"] = string(out)
		Expect(k8sClient.Create(context.Background(), cm)).Should(Succeed())

		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.ClusterSize = pointer.Int32(6)
		hazelcast.Spec.CustomConfigCmName = cm.Name
		CreateHazelcastCR(hazelcast)

		By("split the members into 2 groups")
		podLabels := []PodLabel{
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-0", "group", "group1"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-1", "group", "group1"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-2", "group", "group1"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-3", "group", "group2"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-4", "group", "group2"},
			{"statefulset.kubernetes.io/pod-name=" + hazelcast.Name + "-5", "group", "group2"},
		}
		err = LabelPods(hazelcast.Namespace, podLabels)
		if err != nil {
			fmt.Printf("Error labeling pods: %s\n", err)
		} else {
			fmt.Println("Pods labeled successfully")
		}
		evaluateReadyMembers(hzLookupKey)

		By("run split brain scenario")
		networkPartitionChaos := &chaosmeshv1alpha1.NetworkChaos{
			ObjectMeta: metav1.ObjectMeta{
				Name:      hazelcast.Name,
				Namespace: hazelcast.Namespace,
			},
			Spec: chaosmeshv1alpha1.NetworkChaosSpec{
				PodSelector: chaosmeshv1alpha1.PodSelector{
					Mode: chaosmeshv1alpha1.AllMode,
					Selector: chaosmeshv1alpha1.PodSelectorSpec{
						GenericSelectorSpec: chaosmeshv1alpha1.GenericSelectorSpec{
							LabelSelectors: map[string]string{
								"group": "group1",
							},
						},
					},
				},
				Action:    chaosmeshv1alpha1.PartitionAction,
				Direction: chaosmeshv1alpha1.Both,
				Target: &chaosmeshv1alpha1.PodSelector{
					Mode: chaosmeshv1alpha1.AllMode,
					Selector: chaosmeshv1alpha1.PodSelectorSpec{
						GenericSelectorSpec: chaosmeshv1alpha1.GenericSelectorSpec{
							LabelSelectors: map[string]string{
								"group": "group2",
							},
						},
					},
				},
				Duration: &duration,
			},
		}
		Expect(k8sClient.Create(context.Background(), networkPartitionChaos)).To(Succeed(), "Failed to create network partition chaos")

		By("wait until Hazelcast cluster will be injected by split-brain experiment")
		Eventually(func() bool {
			err = k8sClient.Get(context.Background(), hzLookupKey, networkPartitionChaos)
			Expect(err).ToNot(HaveOccurred())
			for _, condition := range networkPartitionChaos.Status.ChaosStatus.Conditions {
				if condition.Type == chaosmeshv1alpha1.ConditionAllInjected && condition.Status == "True" {
					return true
				}
			}
			return false
		}, 1*Minute, interval).Should(BeTrue())

		By("create a map")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())

		By("attempt to put the 10 entries into the map")
		err = FillMapByEntryCount(context.Background(), hzLookupKey, true, m.MapName(), 10)
		Expect(err).Should(MatchError(MatchRegexp("Split brain protection exception: " + splitBrainConfName + " has failed!")))
	})

	It("should have no data lose after zone outage", Tag(Any), Serial, func() {
		setLabelAndCRName("hr-2")

		ctx := context.Background()
		numberOfNodes, err := numberOfAllNodes(ctx)
		Expect(err).To(BeNil())
		hzClusterSize := numberOfNodes

		By(fmt.Sprintf("creating %d sized cluster with zone-level high availability", hzClusterSize))
		hazelcast := hazelcastconfig.HighAvailability(hzLookupKey, ee, int32(hzClusterSize), "ZONE", labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.BackupCountMap(mapLookupKey, hazelcast.Name, labels, 1)
		Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		mapName := "ha-test-map"
		mapSize := 30000
		err = FillMapByEntryCount(ctx, hzLookupKey, true, mapName, mapSize)
		Expect(err).To(BeNil())
		WaitForMapSize(ctx, hzLookupKey, mapName, mapSize, Minute)

		By("detecting the node which the operator is running on")
		nodeNameOperatorRunningOn, err := nodeNameWhichOperatorRunningOn(ctx)
		Expect(err).To(BeNil())

		By("determining a zone to drop")
		zoneNodeNameMap, err := nodeNamesInZones(ctx)
		Expect(err).To(BeNil())
		var droppingZone string
		for zone, nodeNames := range zoneNodeNameMap {
			safeToDrop := true
			for _, nodeName := range nodeNames {
				safeToDrop = safeToDrop && (nodeName != nodeNameOperatorRunningOn)
			}
			if safeToDrop {
				droppingZone = zone
				break
			}
		}
		numberOfNodesInDroppingZone := len(zoneNodeNameMap[droppingZone])

		By(fmt.Sprintf("dropping zone '%s'", droppingZone))
		err = dropNodes(ctx, droppingZone, zoneNodeNameMap[droppingZone]...)
		Expect(err).To(BeNil())

		By("waiting for nodes to be dropped")
		waitForDroppedNodes(ctx, numberOfNodesInDroppingZone)

		By("checking map size after zone outage")
		WaitForMapSize(ctx, hzLookupKey, mapName, mapSize, Minute)
	})

	It("should have no data lose after node outage", Tag(Any), Serial, func() {
		setLabelAndCRName("hr-1")

		ctx := context.Background()
		numberOfNodes, err := numberOfAllNodes(ctx)
		Expect(err).To(BeNil())
		hzClusterSize := numberOfNodes * 3

		By(fmt.Sprintf("creating %d sized cluster with node-level high availability", hzClusterSize))
		hazelcast := hazelcastconfig.HighAvailability(hzLookupKey, ee, int32(hzClusterSize), "NODE", labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.BackupCountMap(mapLookupKey, hazelcast.Name, labels, 1)
		Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		mapName := "ha-test-map"
		mapSize := 30000
		err = FillMapByEntryCount(ctx, hzLookupKey, true, mapName, mapSize)
		Expect(err).To(BeNil())
		WaitForMapSize(ctx, hzLookupKey, mapName, mapSize, Minute)

		By("detecting the node which the operator is running on")
		nodeNameOperatorRunningOn, err := nodeNameWhichOperatorRunningOn(ctx)
		Expect(err).To(BeNil())

		By("determining a node to drop")
		nodeNames, err := nodeNamesInCluster(ctx)
		Expect(err).To(BeNil())
		var droppingNodeName string
		for _, nodeName := range nodeNames {
			if nodeName != nodeNameOperatorRunningOn {
				droppingNodeName = nodeName
			}
		}

		By(fmt.Sprintf("dropping node '%s'", droppingNodeName))
		var droppingNode corev1.Node
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name: droppingNodeName,
		}, &droppingNode)
		Expect(err).To(BeNil())
		zone, err := zoneFromNodeLabels(&droppingNode)
		Expect(err).To(BeNil())
		err = dropNodes(ctx, zone, droppingNodeName)
		Expect(err).To(BeNil())

		By("waiting the node to be dropped")
		waitForDroppedNodes(ctx, 1)

		By("checking map size after node outage")
		WaitForMapSize(ctx, hzLookupKey, mapName, mapSize, Minute)
	})

	It("should be able to reconnect to Hazelcast cluster upon restart even when Hazelcast cluster is marked to be deleted", Serial, Tag(Any), func() {
		By("clone existing operator")
		setLabelAndCRName("res-1")
		hazelcastSource := hazelcastconfig.Default(hzSrcLookupKey, ee, labels)
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.Default(hzTrgLookupKey, ee, labels)
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)

		evaluateReadyMembers(hzSrcLookupKey)
		evaluateReadyMembers(hzTrgLookupKey)

		By("creating map for source Hazelcast cluster")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcastSource.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("creating wan replication configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			wanLookupKey,
			m.Name,
			hazelcastTarget.Spec.ClusterName,
			hzclient.HazelcastUrl(hazelcastTarget),
			labels,
		)
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())

		By("deleting operator")
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(context.Background(), controllerManagerName, dep)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), dep, client.PropagationPolicy(metav1.DeletePropagationForeground))).Should(Succeed())
		assertDoesNotExist(controllerManagerName, &appsv1.Deployment{})

		By("deleting Hazelcast clusters")
		Expect(k8sClient.Delete(context.Background(), hazelcastSource)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), hazelcastTarget)).Should(Succeed())

		By("deleting wan replication")
		Expect(k8sClient.Delete(context.Background(), wan)).Should(Succeed())

		By("creating operator again")
		newDep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dep.Name,
				Namespace: dep.Namespace,
				Labels:    dep.Labels,
			},
			Spec: dep.Spec,
		}
		Expect(k8sClient.Create(context.Background(), newDep)).Should(Succeed())
		Eventually(func() (int32, error) {
			return getDeploymentReadyReplicas(context.Background(), controllerManagerName, newDep)
		}, 90*Second, interval).Should(Equal(int32(1)))

		assertDoesNotExist(mapLookupKey, m)
		assertDoesNotExist(wanLookupKey, wan)
		assertDoesNotExist(hzSrcLookupKey, hazelcastSource)
		assertDoesNotExist(hzTrgLookupKey, hazelcastTarget)
	})
})

func nodeNamesInCluster(ctx context.Context) ([]string, error) {
	var nodeList corev1.NodeList
	err := k8sClient.List(ctx, &nodeList)
	if err != nil {
		return nil, err
	}
	nodes := nodeList.Items
	nodeNames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	return nodeNames, nil
}

func nodeNamesInZones(ctx context.Context) (map[string][]string, error) {
	var nodeList corev1.NodeList
	err := k8sClient.List(ctx, &nodeList)
	if err != nil {
		return nil, err
	}
	nodes := nodeList.Items
	zoneNodeMap := make(map[string][]string)
	for _, node := range nodes {
		zone, err := zoneFromNodeLabels(&node)
		if err != nil {
			return nil, err
		}
		zoneNodes := zoneNodeMap[zone]
		if zoneNodes == nil {
			zoneNodes = make([]string, 0)
		}
		zoneNodeMap[zone] = append(zoneNodes, node.Name)
	}
	return zoneNodeMap, nil
}

func zoneFromNodeLabels(node *corev1.Node) (string, error) {
	zone, ok := node.Labels[nodeZoneLabel]
	if !ok {
		return "", fmt.Errorf("node '%s' has no '%s' label", node.Name, nodeZoneLabel)
	}
	return zone, nil
}

const nodeZoneLabel = "topology.kubernetes.io/zone"

var gcpProject = os.Getenv("GCP_PROJECT_ID")
var cloudKeySecretName = os.Getenv("CLOUD_KEY_SECRET_NAME")
var chaosMeshNamespace = os.Getenv("CHAOS_MESH_NAMESPACE")

const chaosMeshNodeStopTemplate = `
apiVersion: chaos-mesh.org/v1alpha1
kind: GCPChaos
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
spec:
  action: node-stop
  secretName: {{ .secretName }}
  project: {{ .project }}
  zone: {{ .zone }}
  instance: {{ .instance }}
  duration: '30s'
`

func chaosMeshNodeStopYaml(parameters map[string]string) (string, error) {
	tpl, err := template.New("").Parse(chaosMeshNodeStopTemplate)
	if err != nil {
		return "", err
	}
	buffer := bytes.Buffer{}
	err = tpl.Execute(&buffer, parameters)
	return buffer.String(), err
}

func kubectlApply(ctx context.Context, yaml string) error {
	cmd := exec.New().CommandContext(ctx, "kubectl", "apply", "-f", "-")
	buffer := bytes.NewBufferString(yaml)
	cmd.SetStdin(buffer)
	return cmd.Run()
}

func dropNodes(ctx context.Context, zone string, nodeNames ...string) error {
	parameters := make(map[string]string)
	parameters["zone"] = zone
	parameters["project"] = gcpProject
	parameters["namespace"] = chaosMeshNamespace
	parameters["secretName"] = cloudKeySecretName
	allYamls := make([]string, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		parameters["instance"] = nodeName
		parameters["name"] = fmt.Sprintf("ha-%s-node-stop-%s", hzLookupKey.Name, nodeName)
		yaml, err := chaosMeshNodeStopYaml(parameters)
		if err != nil {
			return err
		}
		allYamls = append(allYamls, yaml)
	}
	yaml := strings.Join(allYamls, "\n---\n")
	return kubectlApply(ctx, yaml)
}

func nodeNameWhichOperatorRunningOn(ctx context.Context) (string, error) {
	pods := &corev1.PodList{}
	err := k8sClient.List(ctx, pods, client.InNamespace(controllerManagerName.Namespace))
	if err != nil {
		return "", err
	}
	podNameLabel := "app.kubernetes.io/name"
	nodeName := ""
	for _, pod := range pods.Items {
		if pod.Labels[podNameLabel] == "hazelcast-platform-operator" {
			nodeName = pod.Spec.NodeName
			break
		}
	}
	if nodeName == "" {
		return "", fmt.Errorf("missing label '%s'", podNameLabel)
	}
	return nodeName, nil
}

func numberOfAllNodes(ctx context.Context) (int, error) {
	var nodeList corev1.NodeList
	err := k8sClient.List(ctx, &nodeList)
	if err != nil {
		return 0, err
	}
	return len(nodeList.Items), nil
}

func numberOfReadyNodes(ctx context.Context) (int, error) {
	var nodeList corev1.NodeList
	err := k8sClient.List(ctx, &nodeList)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, node := range nodeList.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				n++
				break
			}
		}
	}
	return n, nil
}

func waitForDroppedNodes(ctx context.Context, expectedNumberOfDroppedNodes int) {
	numberOfNodes, err := numberOfAllNodes(ctx)
	Expect(err).To(BeNil())
	Eventually(func() int {
		n, err := numberOfReadyNodes(ctx)
		Expect(err).To(BeNil())
		return numberOfNodes - n
	}, 9*Minute, interval).Should(Equal(expectedNumberOfDroppedNodes))
}
