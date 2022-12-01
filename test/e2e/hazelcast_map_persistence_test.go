package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Map Config with Persistence", Label("map_persistence"), func() {
	localPort := strconv.Itoa(8100 + GinkgoParallelProcess())
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
				GinkgoWriter.Printf("%+v\n", controllerManagerName)
				return getDeploymentReadyReplicas(context.Background(), controllerManagerName, controllerDep)
			}, 90*Second, interval).Should(Equal(int32(1)))
		})
	})

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastv1alpha1.HotBackup{}, &hazelcastv1alpha1.HotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Map{}, &hazelcastv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should fail when persistence of Map CR and Hazelcast CR do not match", Label("fast"), func() {
		setLabelAndCRName("hmp-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)

		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastv1alpha1.MapFailed)
		Expect(m.Status.Message).To(Equal(fmt.Sprintf("persistence is not enabled for the Hazelcast resource %s", hazelcast.Name)))
	})

	It("should keep the entries after a Hot Backup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hmp-2")
		clusterSize := int32(3)

		hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1alpha1.MapSuccess)

		By("filling the map with entries")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("creating HotBackup CR")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		assertHotBackupSuccess(hotBackup, 1*Minute)

		By("filling the map with entries after backup")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		RemoveHazelcastCR(hazelcast)

		By("creating new Hazelcast cluster from existing backup")
		hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
		hazelcast.Spec.Persistence.Restore = &hazelcastv1alpha1.RestoreConfiguration{
			HotBackupResourceName: hotBackup.Name,
		}

		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(hazelcast, hazelcastv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 10, 1*Minute)
	})

	It("should persist the map successfully created configs into the configmap", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hmp-3")
		maps := []string{"map1", "map2", "map3", "mapfail"}

		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map configs")
		for i, mapp := range maps {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapp, Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.Eviction = &hazelcastv1alpha1.EvictionConfig{MaxSize: pointer.Int32(int32(i) * 100)}
			m.Spec.HazelcastResourceName = hazelcast.Name
			if mapp == "mapfail" {
				m.Spec.HazelcastResourceName = "failedHz"
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			if mapp == "mapfail" {
				assertMapStatus(m, hazelcastv1alpha1.MapFailed)
				continue
			}
			assertMapStatus(m, hazelcastv1alpha1.MapSuccess)
		}

		By("checking if the maps are in the map config", func() {
			hzConfig := assertMapConfigsPersisted(hazelcast, "map1", "map2", "map3")
			for i, mapp := range maps {
				if mapp != "mapfail" {
					Expect(hzConfig.Hazelcast.Map[mapp].Eviction.Size).Should(Equal(int32(i) * 100))
				}
			}
		})

		By("deleting map2")
		Expect(k8sClient.Delete(context.Background(),
			&hazelcastv1alpha1.Map{ObjectMeta: v1.ObjectMeta{Name: "map2", Namespace: hazelcast.Namespace}})).Should(Succeed())

		By("checking if map2 is not persisted in the configmap", func() {
			_ = assertMapConfigsPersisted(hazelcast, "map1", "map3")
		})
	})

	It("should persist Map Config with Indexes", Label("fast"), func() {
		setLabelAndCRName("hmp-4")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.Indexes = []hazelcastv1alpha1.IndexConfig{
			{
				Name:       "index-1",
				Type:       hazelcastv1alpha1.IndexTypeHash,
				Attributes: []string{"attribute1", "attribute2"},
				BitmapIndexOptions: &hazelcastv1alpha1.BitmapIndexOptionsConfig{
					UniqueKey:           "key",
					UniqueKeyTransition: hazelcastv1alpha1.UniqueKeyTransitionRAW,
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastv1alpha1.MapSuccess)

		By("checking if the map is in the configmap")
		hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)

		By("checking if the indexes are persisted")
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Name).Should(Equal("index-1"))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Type).Should(Equal(string(hazelcastv1alpha1.IndexTypeHash)))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Attributes).Should(ConsistOf("attribute1", "attribute2"))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].BitmapIndexOptions.UniqueKey).Should(Equal("key"))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(string(hazelcastv1alpha1.UniqueKeyTransitionRAW)))
	})

	It("should continue persisting last applied Map Config in case of failure", Label("fast"), func() {
		setLabelAndCRName("hmp-5")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastv1alpha1.MapSuccess)

		By("checking if the map config is persisted")
		hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)
		mcfg := hzConfig.Hazelcast.Map[m.Name]

		By("failing to update the map config")
		m.Spec.BackupCount = pointer.Int32(4)
		Expect(k8sClient.Update(context.Background(), m)).ShouldNot(Succeed())

		By("checking if the same map config is still there")
		// Should wait for Hazelcast reconciler to get triggered, we do not have a waiting mechanism for that.
		Sleep(5 * Second)
		hzConfig = assertMapConfigsPersisted(hazelcast, m.Name)
		newMcfg := hzConfig.Hazelcast.Map[m.Name]
		Expect(newMcfg).To(Equal(mcfg))
	})
})
