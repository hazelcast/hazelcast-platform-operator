package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Platform Resilience Tests", Label("resilience"), func() {
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
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)

		By("waiting for all nodes are ready", func() {
			waitForDroppedNodes(context.Background(), 0)
		})

		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should have no data lose after node outage", Label("slow"), func() {
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
		FillMapByEntryCount(ctx, hzLookupKey, true, mapName, mapSize)
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

	It("should have no data lose after zone outage", Label("slow"), func() {
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
		FillMapByEntryCount(ctx, hzLookupKey, true, mapName, mapSize)
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
