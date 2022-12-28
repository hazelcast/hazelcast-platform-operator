package e2e

import (
	"context"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast WAN", Label("hz_wan"), func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

	waitForLBAddress := func(name types.NamespacedName) string {
		By("waiting for load balancer address")
		hz := &hazelcastcomv1alpha1.Hazelcast{}
		Eventually(func() string {
			hz := &hazelcastcomv1alpha1.Hazelcast{}
			err := k8sClient.Get(context.Background(), name, hz)
			Expect(err).ToNot(HaveOccurred())
			return hz.Status.ExternalAddresses
		}, 3*Minute, interval).Should(Not(BeEmpty()))
		Expect(k8sClient.Get(context.Background(), name, hz)).To(Succeed())
		return hz.Status.ExternalAddresses
	}

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
		DeleteAllOf(&hazelcastcomv1alpha1.WanReplication{}, &hazelcastcomv1alpha1.WanReplicationList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should send data to another cluster", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hw-1")

		By("creating source Hazelcast cluster")
		hazelcastSource := hazelcastconfig.ExposeExternallyUnisocket(hzSrcLookupKey, ee, labels)
		hazelcastSource.Spec.ClusterName = "source"
		CreateHazelcastCR(hazelcastSource)

		By("creating target Hazelcast cluster")
		hazelcastTarget := hazelcastconfig.ExposeExternallyUnisocket(hzTrgLookupKey, ee, labels)
		hazelcastTarget.Spec.ClusterName = "target"
		CreateHazelcastCR(hazelcastTarget)
		evaluateReadyMembers(hzSrcLookupKey)
		evaluateReadyMembers(hzTrgLookupKey)

		_ = waitForLBAddress(hzSrcLookupKey)
		targetAddress := waitForLBAddress(hzTrgLookupKey)

		By("creating map for source Hazelcast cluster")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcastSource.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("creating WAN configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			wanLookupKey,
			m.Name,
			hazelcastTarget.Spec.ClusterName,
			targetAddress,
			labels,
		)
		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())

		Eventually(func() (hazelcastcomv1alpha1.WanStatus, error) {
			wan := &hazelcastcomv1alpha1.WanReplication{}
			err := k8sClient.Get(context.Background(), wanLookupKey, wan)
			if err != nil {
				return hazelcastcomv1alpha1.WanStatusFailed, err
			}
			return wan.Status.Status, nil
		}, 30*Second, interval).Should(Equal(hazelcastcomv1alpha1.WanStatusSuccess))

		mapSize := 1024
		FillTheMapData(context.Background(), hzSrcLookupKey, true, m.Name, mapSize)

		By("checking the size of the map in the target cluster")
		WaitForMapSize(context.Background(), hzTrgLookupKey, m.Name, mapSize, 1*Minute)
	})

	It("should replicate multiple maps to target cluster ", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hw-2")
		source1LookupKey := types.NamespacedName{Name: hzSrcLookupKey.Name + "1", Namespace: hzSrcLookupKey.Namespace}
		source2LookupKey := types.NamespacedName{Name: hzSrcLookupKey.Name + "2", Namespace: hzSrcLookupKey.Namespace}

		By("creating first source Hazelcast cluster")
		source1 := hazelcastconfig.Default(source1LookupKey, ee, labels)
		source1.Spec.ClusterName = "source1"
		source1.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCR(source1)

		By("creating second source Hazelcast cluster")
		source2 := hazelcastconfig.Default(source2LookupKey, ee, labels)
		source2.Spec.ClusterName = "source2"
		source2.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCR(source2)

		By("creating target Hazelcast cluster")
		target1 := hazelcastconfig.Default(hzTrgLookupKey, ee, labels)
		target1.Spec.ClusterName = "target"
		target1.Spec.ClusterSize = pointer.Int32(1)
		CreateHazelcastCR(target1)

		evaluateReadyMembers(source1LookupKey)
		evaluateReadyMembers(source2LookupKey)
		evaluateReadyMembers(hzTrgLookupKey)

		By("creating first map for first source Hazelcast cluster")
		source1_map1 := hazelcastconfig.DefaultMap(mapLookupKey, source1.Name, labels)
		source1_map1.Name = "source1-map1"
		Expect(k8sClient.Create(context.Background(), source1_map1)).Should(Succeed())
		_ = assertMapStatus(source1_map1, hazelcastcomv1alpha1.MapSuccess)

		By("creating second map for first source Hazelcast cluster")
		source1_map2 := hazelcastconfig.DefaultMap(mapLookupKey, source1.Name, labels)
		source1_map2.Name = "source1-map2"
		Expect(k8sClient.Create(context.Background(), source1_map2)).Should(Succeed())
		_ = assertMapStatus(source1_map2, hazelcastcomv1alpha1.MapSuccess)

		By("creating first map for second source Hazelcast cluster")
		source2_map1 := hazelcastconfig.DefaultMap(mapLookupKey, source2.Name, labels)
		source2_map1.Name = "source2-map1"
		Expect(k8sClient.Create(context.Background(), source2_map1)).Should(Succeed())
		_ = assertMapStatus(source2_map1, hazelcastcomv1alpha1.MapSuccess)

		By("creating first WAN configuration")
		wan := hazelcastconfig.DefaultWanReplication(
			wanLookupKey,
			source1_map1.Name,
			target1.Spec.ClusterName,
			hzclient.HazelcastUrl(target1),
			labels,
		)

		wan.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
			{Name: source1_map1.Name},
			{Name: source2.Name, Kind: hazelcastcomv1alpha1.ResourceKindHZ},
		}

		Expect(k8sClient.Create(context.Background(), wan)).Should(Succeed())
		assertWanStatus(wan, hazelcastcomv1alpha1.WanStatusSuccess)

		By("creating second WAN configuration")
		wlk := types.NamespacedName{Namespace: wan.Namespace, Name: "second-wan"}
		wan2 := hazelcastconfig.DefaultWanReplication(
			wlk,
			source1_map1.Name,
			target1.Spec.ClusterName,
			hzclient.HazelcastUrl(target1),
			labels,
		)

		wan2.Spec.Resources = []hazelcastcomv1alpha1.ResourceSpec{
			{Name: source1_map2.Name},
		}

		Expect(k8sClient.Create(context.Background(), wan2)).Should(Succeed())
		assertWanStatus(wan2, hazelcastcomv1alpha1.WanStatusSuccess)

		By("filling the maps in the source clusters")
		mapSize := 10
		fillTheMapDataPortForward(context.Background(), source1, localPort, source1_map1.Name, mapSize)
		fillTheMapDataPortForward(context.Background(), source1, localPort, source1_map2.Name, mapSize)
		fillTheMapDataPortForward(context.Background(), source2, localPort, source2_map1.Name, mapSize)

		By("checking the size of the maps in the target cluster")
		waitForMapSizePortForward(context.Background(), target1, localPort, source1_map1.Name, mapSize, 1*Minute)
		waitForMapSizePortForward(context.Background(), target1, localPort, source1_map2.Name, mapSize, 1*Minute)
		waitForMapSizePortForward(context.Background(), target1, localPort, source2_map1.Name, mapSize, 1*Minute)
	})
})
