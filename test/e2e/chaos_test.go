package e2e

import (
	"context"
	"fmt"
	chaosmeshv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
	cli "sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	. "time"
)

type PodLabel struct {
	Selector   string
	LabelKey   string
	LabelValue string
}

var _ = Describe("Hazelcast Chaos Tests", Label("chaos_tests"), func() {

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
		GinkgoWriter.Printf("AfterEach end time is %v\n", Now().String())
	})

	It("should kill the pod randomly and preserve the data after restore", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("ct-1")
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
		CreateMaps(context.Background(), nMaps, hazelcast.Name, hazelcast)

		By("put the entries")
		FillMaps(context.Background(), nMaps, mapSizeInMb, hazelcast.Name, mapSizeInMb, hazelcast)

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
			newPods, err := getClientSet().CoreV1().Pods(hazelcast.Namespace).List(context.TODO(), metav1.ListOptions{
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

	It("should check a split-brain protection in the Hazelcast cluster", Label("fast"), func() {
		setLabelAndCRName("ct-2")
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
		err = FillTheMapData(context.Background(), hzLookupKey, true, m.MapName(), 10)
		Expect(err).Should(MatchError(MatchRegexp("Split brain protection exception: " + splitBrainConfName + " has failed!")))
	})
})
