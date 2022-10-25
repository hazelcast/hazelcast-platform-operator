package e2e

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hzTypes "github.com/hazelcast/hazelcast-go-client/types"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast User Code Deployment", Label("custom_class"), func() {
	localPort := strconv.Itoa(8200 + GinkgoParallelProcess())

	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("checking hazelcast-platform-controller-manager running", func() {
			controllerDep := &appsv1.Deployment{}
			Eventually(func() (int32, error) {
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&corev1.Secret{}, &corev1.SecretList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should use the MapStore implementation correctly", Label("fast"), func() {
		setLabelAndCRName("hmcc-1")
		propSecretName := "prop-secret"
		msClassName := "SimpleStore"

		By("creating the Hazelcast CR")
		hazelcast := hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/mapStore", labels)
		CreateHazelcastCR(hazelcast)

		By("port-forwarding to Hazelcast master pod")
		stopChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		By("creating mapStore properties secret")
		secretData := map[string]string{"username": "user1", "password": "pass1"}
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: propSecretName, Namespace: hazelcast.Namespace, Labels: labels}, StringData: secretData}
		Expect(k8sClient.Create(context.Background(), s)).Should(Succeed())

		By("creating map with MapStore")
		ms := hazelcastcomv1alpha1.MapSpec{
			HazelcastResourceName: hzLookupKey.Name,
			MapStore: &hazelcastcomv1alpha1.MapStoreConfig{
				ClassName:            msClassName,
				PropertiesSecretName: propSecretName,
			},
		}
		m := hazelcastconfig.Map(ms, mapLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		t := Now()

		By("filling the map with entries")
		entryCount := 5
		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			Expect(cl.Shutdown(context.Background())).Should(Succeed())
		}()
		mp, err := cl.GetMap(context.Background(), m.MapName())
		Expect(err).To(BeNil())

		entries := make([]hzTypes.Entry, entryCount)
		for i := 0; i < entryCount; i++ {
			entries[i] = hzTypes.NewEntry(strconv.Itoa(i), "val")
		}
		err = mp.PutAll(context.Background(), entries...)
		Expect(err).To(BeNil())
		Expect(mp.Size(context.Background())).Should(Equal(entryCount))

		By("checking the logs")
		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("SimpleStore - Properties are"))
		line := scanner.Text()
		for k, v := range secretData {
			Expect(line).To(ContainSubstring(k + "=" + v))
		}
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring(fmt.Sprintf("SimpleStore - Map name is %s", m.MapName())))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring("SimpleStore - loading all keys"))
		test.EventuallyInLogs(scanner, 10*Second, logInterval).Should(ContainSubstring(fmt.Sprintf("SimpleStore - storing key: %d", entryCount-1)))

	})

	It("should add executor services both initially and dynamically", Label("fast"), func() {
		setLabelAndCRName("hcc-1")

		executorServices := []hazelcastcomv1alpha1.ExecutorServiceConfiguration{
			{
				Name:     "service1",
				PoolSize: 5,
			},
		}
		durableExecutorServices := []hazelcastcomv1alpha1.DurableExecutorServiceConfiguration{
			{
				Name:       "service1",
				Durability: 20,
			},
		}
		scheduledExecutorServices := []hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration{
			{
				Name:           "service2",
				CapacityPolicy: "PER_PARTITION",
			},
		}
		sampleExecutorServices := map[string]interface{}{"es": executorServices, "des": durableExecutorServices, "ses": scheduledExecutorServices}

		By("creating the Hazelcast CR")
		hazelcast := hazelcastconfig.ExecutorService(hzLookupKey, ee, sampleExecutorServices, labels)
		CreateHazelcastCR(hazelcast)

		By("port-forwarding to Hazelcast master pod")
		stopChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		By("checking if the initially added executor service configs are created correctly")
		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			Expect(cl.Shutdown(context.Background())).Should(Succeed())
		}()

		memberConfigXML := getMemberConfig(context.Background(), cl)
		actualES := getExecutorServiceConfigFromMemberConfig(memberConfigXML)
		assertExecutorServices(sampleExecutorServices, actualES)

		By("adding new executor services dynamically")
		sampleExecutorServices["es"] = append(sampleExecutorServices["es"].([]hazelcastcomv1alpha1.ExecutorServiceConfiguration), hazelcastcomv1alpha1.ExecutorServiceConfiguration{Name: "new-service", QueueCapacity: 50})
		sampleExecutorServices["des"] = append(sampleExecutorServices["des"].([]hazelcastcomv1alpha1.DurableExecutorServiceConfiguration), hazelcastcomv1alpha1.DurableExecutorServiceConfiguration{Name: "new-durable-service", PoolSize: 12, Capacity: 40})
		sampleExecutorServices["ses"] = append(sampleExecutorServices["ses"].([]hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration), hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration{Name: "new-scheduled-service", PoolSize: 12, Capacity: 40})

		UpdateHazelcastCR(hazelcast, func(hz *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hz.Spec.ExecutorServices = sampleExecutorServices["es"].([]hazelcastcomv1alpha1.ExecutorServiceConfiguration)
			hz.Spec.DurableExecutorServices = sampleExecutorServices["des"].([]hazelcastcomv1alpha1.DurableExecutorServiceConfiguration)
			hz.Spec.ScheduledExecutorServices = sampleExecutorServices["ses"].([]hazelcastcomv1alpha1.ScheduledExecutorServiceConfiguration)
			return hz
		})

		By("checking if all the executor service configs are created correctly", func() {
			Eventually(func() []int {
				memberConfigXML = getMemberConfig(context.Background(), cl)
				actualES = getExecutorServiceConfigFromMemberConfig(memberConfigXML)
				return []int{len(actualES.Basic), len(actualES.Durable), len(actualES.Scheduled)}
			}, 90*Second, interval).Should(Equal([]int{3, 2, 2}))
		})

		assertExecutorServices(sampleExecutorServices, actualES)
	})

	It("should add entry listeners", Label("fast"), func() {
		setLabelAndCRName("hel-1")

		h := hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/entryListener", labels)
		CreateHazelcastCR(h)
		t := Now()

		By("creating map with Map with entry listener")
		ms := hazelcastcomv1alpha1.MapSpec{
			HazelcastResourceName: hzLookupKey.Name,
			EntryListeners: []hazelcastcomv1alpha1.EntryListenerConfiguration{
				{
					ClassName: "org.example.SampleEntryListener",
				},
			},
		}
		m := hazelcastconfig.Map(ms, mapLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("port-forwarding to Hazelcast master pod")
		stopChan := portForwardPod(hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/entryListener", labels).Name+"-0", hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/mapStore", labels).Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		By("filling the map with entries")
		entryCount := 5
		cl := createHazelcastClient(context.Background(), hazelcastconfig.UserCode(hzLookupKey, ee, "br-secret-gcp", "gs://operator-user-code/entryListener", labels), localPort)
		defer func() {
			Expect(cl.Shutdown(context.Background())).Should(Succeed())
		}()
		mp, err := cl.GetMap(context.Background(), m.MapName())
		Expect(err).To(BeNil())

		entries := make([]hzTypes.Entry, entryCount)
		for i := 0; i < entryCount; i++ {
			entries[i] = hzTypes.NewEntry(strconv.Itoa(i), "val")
		}
		err = mp.PutAll(context.Background(), entries...)
		Expect(err).To(BeNil())
		Expect(mp.Size(context.Background())).Should(Equal(entryCount))

		By("checking the logs")
		logs := InitLogs(t, hzLookupKey)
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		for _, e := range entries {
			test.EventuallyInLogs(scanner, 10*Second, logInterval).
				Should(ContainSubstring(fmt.Sprintf("EntryAdded, key: %s, value:%s", e.Key, e.Value)))
		}
	})

})
