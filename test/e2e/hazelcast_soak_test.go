package e2e

import (
	"context"
	"fmt"
	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	mcconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/managementcenter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"os"
	"strconv"
	. "time"
)

var _ = Describe("Hazelcast High Load Tests", Label("soak"), func() {
	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.ManagementCenter{}, nil, hzNamespace, labels)

		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should upgrade HZ version after pause/resume with default partition count during 4 hours and keep 45 GB data", Serial, Label("slow"), func() {

		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("soak-1")
		var pvcSizeInMb = 14500
		var pauseBetweenFills = 4 * Minute
		var initHzVersion = "5.2.4"
		var updatedHzVersion = os.Getenv("HZ_VERSION")

		var mapSizeInMb = 24
		var numMaps = 10
		var totalFillRepeats = 6
		var totalPauseResumeCycles = 10
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
		mc.Spec.HazelcastClusters = []hazelcastcomv1alpha1.HazelcastClusterConfig{
			{Name: "dev", Address: hzLookupKey.Name},
		}

		create(mc)

		By("creating Hazelcast cluster with partition count and 10 maps")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Name = hzLookupKey.Name
		hazelcast.Spec.Version = initHzVersion
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

		By("create the map config")
		CreateMaps(ctx, numMaps, hazelcast.Name, hazelcast)
		sizeForCycleInMb := 0
		for cycle := 1; cycle <= totalPauseResumeCycles; cycle++ {
			By(fmt.Sprintf("putting %d entries", mapSizeInMb*128))
			for repeat := 1; repeat <= totalFillRepeats; repeat++ {
				sizeForRepeat := sizeForCycleInMb + repeat*mapSizeInMb
				By(fmt.Sprintf("expected total entries for repeat %d in cycle %d must be: %d\n", repeat, cycle, sizeForRepeat*128))
				FillMaps(ctx, numMaps, mapSizeInMb, hazelcast.Name, sizeForRepeat, hazelcast)
				Sleep(pauseBetweenFills)
			}
			sizeForCycleInMb += totalFillRepeats * mapSizeInMb
			By(fmt.Sprintf("Expected total entries for the %d cycle must be: %d\n", cycle, sizeForCycleInMb*128))

			By(fmt.Sprintf("pause Hazelcast for the cycle %d\n:", cycle))
			UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				hazelcast.Spec.ClusterSize = pointer.Int32(0)
				return hazelcast
			})
			WaitForReplicaSize(hazelcast.Namespace, hazelcast.Name, 0)

			By(fmt.Sprintf("pause Management Center for the cycle %d\n:", cycle))
			ScaleStatefulSet(mc.Namespace, mc.Name, 0)

			By(fmt.Sprintf("resume Management Center for the cycle %d\n:", cycle))
			ScaleStatefulSet(mc.Namespace, mc.Name, 1)

			By(fmt.Sprintf("resume Hazelcast for the cycle %d\n:", cycle))
			UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
				var hzVersion string
				hazelcast.Spec.ClusterSize = pointer.Int32(3)
				if cycle > 5 {
					hzVersion = updatedHzVersion
				} else {
					hzVersion = initHzVersion
				}
				hazelcast.Spec.Version = hzVersion
				return hazelcast
			})
			evaluateReadyMembers(hzLookupKey)

			By(fmt.Sprintf("checking HZ status after resume for the cycle %d\n:", cycle))
			Eventually(func() hazelcastcomv1alpha1.Phase {
				err := k8sClient.Get(ctx, hzLookupKey, hazelcast)
				Expect(err).ToNot(HaveOccurred())
				return hazelcast.Status.Phase
			}, 5*Minute, interval).ShouldNot(Equal(hazelcastcomv1alpha1.Pending))

			By(fmt.Sprintf("checking map size after %d pause/resume cycle and %d total fill repeats", cycle, totalFillRepeats))
			for i := 0; i < numMaps; i++ {
				m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, hazelcast.Name), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
				m.Spec.HazelcastResourceName = hazelcast.Name
				WaitForMapSize(ctx, hzLookupKey, m.Name, int(float64(cycle*totalFillRepeats*mapSizeInMb)*128), 2*Minute)
			}
		}
		By(fmt.Sprintf("checking the map size after %d total pause/resume cycles and %d total fill repeats", totalPauseResumeCycles, totalFillRepeats))
		for i := 0; i < numMaps; i++ {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, hazelcast.Name), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			WaitForMapSize(ctx, hzLookupKey, m.Name, int(float64(totalPauseResumeCycles*totalFillRepeats*mapSizeInMb)*128), 2*Minute)
		}
	})
})
