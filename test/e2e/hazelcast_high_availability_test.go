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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast High Availability", Label("hz_ha"), func() {
	localPort := strconv.Itoa(8100 + GinkgoParallelProcess())

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		ctx := context.Background()
		By("checking hazelcast-platform-operator running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(ctx, controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))

		})

		By("checking chaos-mesh-operator running", func() {
			podNames, err := allPodNamesInNamespace(ctx, chaosMeshNamespace)
			Expect(err).To(BeNil())
			Expect(podNames).Should(ContainElement(ContainSubstring("chaos-controller-manager")))
			Expect(podNames).Should(ContainElement(ContainSubstring("chaos-daemon")))

		})

		By("checking Kubernetes topology is proper for running zone-level tests", func() {
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
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)

		By("deleting all ChaosMesh GCPChaos resources", func() {
			err := deleteChaosMeshAllGCPChaosResources(context.Background(), "chaos-mesh")
			Expect(err).To(BeNil())
		})

		By("waiting for all nodes are ready", func() {
			waitForDroppedNodes(context.Background(), 0)
		})

		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should have no data lose after node outage", Label("slow"), func() {
		setLabelAndCRName("hha-1")

		ctx := context.Background()
		numberOfNodes, err := numberOfAllNodes(ctx)
		Expect(err).To(BeNil())
		hzClusterSize := numberOfNodes * 3

		By(fmt.Sprintf("creating %d sized cluster with node-level high availability", hzClusterSize))
		hazelcast := hazelcastconfig.HighAvailability(hzLookupKey, ee, int32(hzClusterSize), "NODE", labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		mapSize := 1000
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), mapSize)

		By("checking map size before zone outage")
		mapSizeBeforeDropping := mapSizePortForward(ctx, hazelcast, fmt.Sprintf("%s-%d", hazelcast.Name, 0), localPort, m.MapName())
		Expect(mapSizeBeforeDropping).Should(Equal(mapSize))

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
		zone, err := zoneOfNode(ctx, droppingNodeName)
		Expect(err).To(BeNil())
		err = dropNodes(ctx, zone, droppingNodeName)
		Expect(err).To(BeNil())

		By("waiting the node to be dropped")
		waitForDroppedNodes(ctx, 1)

		By("checking map size after node outage")
		nodeHzMemberPodNameMap, err := hzMemberPodsNamesInNodes(ctx, hazelcast.Name)
		Expect(err).To(BeNil())
		var runningHzMemberPodName string
		for _, nodeName := range nodeNames {
			if nodeName == droppingNodeName {
				continue
			}
			pods, ok := nodeHzMemberPodNameMap[nodeName]
			if ok && len(pods) > 0 {
				runningHzMemberPodName = pods[0]
				break
			}
		}
		mapSizeAfterDropping := mapSizePortForward(ctx, hazelcast, runningHzMemberPodName, localPort, m.MapName())
		Expect(mapSizeAfterDropping).Should(Equal(mapSize))

	})

	It("should have no data lose after zone outage", Label("slow"), func() {
		setLabelAndCRName("hha-2")

		ctx := context.Background()
		numberOfNodes, err := numberOfAllNodes(ctx)
		Expect(err).To(BeNil())
		hzClusterSize := numberOfNodes

		By(fmt.Sprintf("creating %d sized cluster with zone-level high availability", hzClusterSize))
		hazelcast := hazelcastconfig.HighAvailability(hzLookupKey, ee, int32(hzClusterSize), "ZONE", labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		mapSize := 1000
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), mapSize)

		By("checking map size before zone outage")
		mapSizeBeforeDropping := mapSizePortForward(ctx, hazelcast, fmt.Sprintf("%s-%d", hazelcast.Name, 0), localPort, m.MapName())
		Expect(mapSizeBeforeDropping).Should(Equal(mapSize))

		By("detecting the node which the operator is running on")
		nodeNameOperatorRunningOn, err := nodeNameWhichOperatorRunningOn(ctx)
		Expect(err).To(BeNil())

		By("determining a zone to drop")
		zoneNodeNameMap, err := nodeNamesInZones(ctx)
		Expect(err).To(BeNil())
		var droppingZone string
	zoneNodeNames:
		for zone, nodeNames := range zoneNodeNameMap {
			for _, nodeName := range nodeNames {
				if nodeName == nodeNameOperatorRunningOn {
					continue zoneNodeNames
				}
			}
			droppingZone = zone
			break
		}
		numberOfNodesInDroppingZone := len(zoneNodeNameMap[droppingZone])

		By(fmt.Sprintf("dropping zone '%s'", droppingZone))
		err = dropNodes(ctx, droppingZone, zoneNodeNameMap[droppingZone]...)
		Expect(err).To(BeNil())

		By("waiting for nodes to be dropped")
		waitForDroppedNodes(ctx, numberOfNodesInDroppingZone)

		By("checking map size after zone outage")
		nodeHzMemberPodNameMap, err := hzMemberPodsNamesInNodes(ctx, hazelcast.Name)
		Expect(err).To(BeNil())
		var runningHzMemberPodName string
		for zone, nodeNames := range zoneNodeNameMap {
			if zone == droppingZone {
				continue
			}
			for _, nodeName := range nodeNames {
				pods, ok := nodeHzMemberPodNameMap[nodeName]
				if ok && len(pods) > 0 {
					runningHzMemberPodName = pods[0]
					break
				}
			}
			if runningHzMemberPodName != "" {
				break
			}
		}
		mapSizeAfterDropping := mapSizePortForward(ctx, hazelcast, runningHzMemberPodName, localPort, m.MapName())
		Expect(mapSizeAfterDropping).Should(Equal(mapSize))
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
  duration: '5m'
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

func deleteChaosMeshAllGCPChaosResources(ctx context.Context, namespace string) error {
	cmd := exec.New().CommandContext(ctx,
		"kubectl", "delete", "GCPChaos", "--all", fmt.Sprintf("-n=%s", namespace))
	return cmd.Run()
}

func appendYamls(yamls ...string) string {
	sb := strings.Builder{}
	for i, yaml := range yamls {
		sb.WriteString(yaml)
		if i < len(yamls)-1 {
			sb.WriteString("\n---\n")
		}
	}
	return sb.String()
}

func dropNodes(ctx context.Context, zone string, nodeNames ...string) error {
	parameters := make(map[string]string)
	parameters["zone"] = zone
	parameters["project"] = gcpProject
	parameters["namespace"] = chaosMeshNamespace
	parameters["secretName"] = cloudKeySecretName
	yamls := make([]string, 0, len(nodeNames))
	for _, nodeName := range nodeNames {
		parameters["instance"] = nodeName
		parameters["name"] = fmt.Sprintf("node-stop-%s", nodeName)
		yaml, err := chaosMeshNodeStopYaml(parameters)
		if err != nil {
			return err
		}
		yamls = append(yamls, yaml)
	}
	return kubectlApply(ctx, appendYamls(yamls...))
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

func hzMemberPodsNamesInNodes(ctx context.Context, hzName string) (map[string][]string, error) {
	var podList corev1.PodList
	err := k8sClient.List(ctx, &podList)
	if err != nil {
		return nil, err
	}
	nodePodMap := make(map[string][]string)
	for _, pod := range podList.Items {
		if !strings.HasPrefix(pod.Name, hzName) {
			continue
		}
		pods := nodePodMap[pod.Spec.NodeName]
		if pods == nil {
			pods = make([]string, 0)
		}
		nodePodMap[pod.Spec.NodeName] = append(pods, pod.Name)
	}
	return nodePodMap, nil
}

func zoneOfNode(ctx context.Context, nodeName string) (string, error) {
	var node corev1.Node
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name: nodeName,
	}, &node)
	if err != nil {
		return "", err
	}
	return zoneFromNodeLabels(&node)
}

func allPodNamesInNamespace(ctx context.Context, namespace string) ([]string, error) {
	var podList corev1.PodList
	err := k8sClient.List(ctx, &podList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}
	podNames := make([]string, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}
	return podNames, err
}
