package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("Platform Rolling UpgradeTests", Group("rolling_upgrade"), func() {
	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.ManagementCenter{}, nil, hzNamespace, labels)

		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should upgrade HZ version after pause/resume with 7999 partition count", Serial, Tag(Slow), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hra-1")
		var mapSizeInMb = 500
		var pvcSizeInMb = 14500
		var numMaps = 28
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()
		clusterSize := int32(3)

		create := func(mancenter *hazelcastcomv1alpha1.ManagementCenter) {
			By("creating ManagementCenter CR", func() {
				Expect(k8sClient.Create(context.Background(), mancenter)).Should(Succeed())
			})

			By("checking ManagementCenter CR running", func() {
				mc := &hazelcastcomv1alpha1.ManagementCenter{}
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), mcLookupKey, mc)
					Expect(err).ToNot(HaveOccurred())
					return isManagementCenterRunning(mc)
				}, 5*Minute, interval).Should(BeTrue())
			})
		}

		mc := mcconfig.Default(mcLookupKey, ee, labels)
		mc.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse("1Gi")},
		}
		mc.Spec.Persistence = &hazelcastcomv1alpha1.MCPersistenceConfiguration{
			Size: &[]resource.Quantity{resource.MustParse("5Gi")}[0]}

		create(mc)

		By("creating Hazelcast cluster with partition count 7999 and 28 maps")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Version = "5.2.4"
		jvmArgs := []string{
			"-Dhazelcast.partition.count=7999",
		}
		hazelcast.Spec.JVM = &hazelcastcomv1alpha1.JVMConfiguration{
			Args: jvmArgs,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
		hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("create the map config and put the entries")

		ConcurrentlyCreateAndFillMultipleMapsByMb(ctx, numMaps, mapSizeInMb, hazelcast.Name, hazelcast)

		By("pause Hazelcast")
		UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(0)
			return hazelcast
		})
		WaitForReplicaSize(hazelcast.Namespace, hazelcast.Name, 0)

		By("pause Management Center")
		ScaleStatefulSet(mc.Namespace, mc.Name, 0)

		By("resume Management Center")
		ScaleStatefulSet(mc.Namespace, mc.Name, 1)

		By("resume Hazelcast")
		UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(3)
			hazelcast.Spec.Version = "5.3.2"
			return hazelcast
		})
		evaluateReadyMembers(hzLookupKey)

		By("checking HZ status after resume")
		Eventually(func() hazelcastcomv1alpha1.Phase {
			err := k8sClient.Get(ctx, hzLookupKey, hazelcast)
			Expect(err).ToNot(HaveOccurred())
			return hazelcast.Status.Phase
		}, 5*Minute, interval).ShouldNot(Equal(hazelcastcomv1alpha1.Pending))

		By("checking map size after pause and resume")
		for i := 0; i < numMaps; i++ {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, hazelcast.Name), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			WaitForMapSize(ctx, hzLookupKey, m.MapName(), expectedMapSize, 5*Minute)
		}
	})
})
