package e2e

import (
	"context"
	"fmt"
	"log"
	"strconv"
	. "time"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Platform Rollout Restart Tests", Label("rollout_restart"), func() {
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

	It("should perform rollout restart with 14Gb data", Serial, Tag(EE|AnyCloud), func() {
		setLabelAndCRName("hrr-1")
		var mapSizeInMb = 500
		var pvcSizeInMb = 14500
		var numMaps = 28
		var expectedMapSize = int(float64(mapSizeInMb) * 128)
		ctx := context.Background()
		clusterSize := int32(3)
		By("creating Hazelcast cluster")
		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		hazelcast.Spec.Resources = &corev1.ResourceRequirements{
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
		}
		hazelcast.Spec.Persistence.PVC.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("create the map config and put entries")
		ConcurrentlyCreateAndFillMultipleMapsByMb(ctx, numMaps, mapSizeInMb, hazelcast.Name, hazelcast)

		By("making rollout StatefulSet restart")
		err := RolloutRestart(ctx, hazelcast)
		if err != nil {
			log.Fatalf("Failed to perform rollout restart: %v", err)
		}

		By("checking HZ status after rollout restart")
		Eventually(func() hazelcastcomv1alpha1.Phase {
			err := k8sClient.Get(ctx, hzLookupKey, hazelcast)
			Expect(err).ToNot(HaveOccurred())
			return hazelcast.Status.Phase
		}, 10*Minute, interval).ShouldNot(Equal(hazelcastcomv1alpha1.Pending))

		By("checking map size after rollout restart")
		for i := 0; i < numMaps; i++ {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: fmt.Sprintf("map-%d-%s", i, hazelcast.Name), Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.HazelcastResourceName = hazelcast.Name
			WaitForMapSize(ctx, hzLookupKey, m.MapName(), expectedMapSize, 5*Minute)
		}
	})
})
