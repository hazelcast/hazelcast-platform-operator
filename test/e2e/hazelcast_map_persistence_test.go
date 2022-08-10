package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	hzTypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast Map Config with Persistence", Label("map_persistence"), func() {
	localPort := strconv.Itoa(8000 + GinkgoParallelProcess())
	BeforeEach(func() {
		if !useExistingCluster() {
			Skip("End to end tests require k8s cluster. Set USE_EXISTING_CLUSTER=true")
		}
		if runningLocally() {
			return
		}
		By("Checking hazelcast-platform-controller-manager running", func() {
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
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	It("should fail when persistence of Map CR and Hazelcast CR do not match", Label("fast"), func() {
		setLabelAndCRName("hmp-1")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true

		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
		Expect(m.Status.Message).To(Equal(fmt.Sprintf("persistence is not enabled for the Hazelcast resource %s", hazelcast.Name)))
	})

	It("should keep the entries after a Hot Backup", Label("slow"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hmp-2")
		baseDir := "/data/hot-restart"

		hazelcast := hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("port-forwarding to Hazelcast master pod")
		stopChan, readyChan := portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		err := waitForReadyChannel(readyChan, 5*Second)
		Expect(err).To(BeNil())

		By("creating the map config successfully")
		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.PersistenceEnabled = true
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("Filling the map with entries")
		entryCount := 100
		cl := createHazelcastClient(context.Background(), hazelcast, localPort)
		mp, err := cl.GetMap(context.Background(), m.Name)
		Expect(err).To(BeNil())

		entries := make([]hzTypes.Entry, entryCount)
		for i := 0; i < entryCount; i++ {
			entries[i] = hzTypes.NewEntry(i, "val")
		}
		err = mp.PutAll(context.Background(), entries...)
		Expect(err).To(BeNil())
		Expect(mp.Size(context.Background())).Should(Equal(entryCount))

		By("Shutting down the connection to cluster")
		err = cl.Shutdown(context.Background())
		Expect(err).To(BeNil())
		closeChannel(stopChan)

		By("Creating HotBackup CR")
		t := Now()
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

		By("Wait for backup to finish")
		hb := &hazelcastcomv1alpha1.HotBackup{}
		Eventually(func() hazelcastcomv1alpha1.HotBackupState {
			err := k8sClient.Get(
				context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
			Expect(err).ToNot(HaveOccurred())
			Expect(hb.Status.State).ShouldNot(Equal(hazelcastcomv1alpha1.HotBackupFailure), "Message: %v", hb.Status.Message)
			return hb.Status.State
		}, 10*Minute, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupSuccess))

		seq := GetBackupSequence(t, hzLookupKey)
		RemoveHazelcastCR(hazelcast)

		By("Creating new Hazelcast cluster from existing backup")
		baseDir += "/hot-backup/backup-" + seq
		hazelcast = hazelcastconfig.PersistenceEnabled(hzLookupKey, baseDir, labels)

		Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
		evaluateReadyMembers(hzLookupKey, 3)
		assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)

		By("port-forwarding to restarted Hazelcast master pod")
		stopChan, readyChan = portForwardPod(hazelcast.Name+"-0", hazelcast.Namespace, localPort+":5701")
		defer closeChannel(stopChan)
		err = waitForReadyChannel(readyChan, 5*Second)
		Expect(err).To(BeNil())

		By("Checking the map entries")
		cl = createHazelcastClient(context.Background(), hazelcast, localPort)
		defer func() {
			err := cl.Shutdown(context.Background())
			Expect(err).To(BeNil())
		}()
		mp, err = cl.GetMap(context.Background(), m.Name)
		Expect(err).To(BeNil())
		Expect(mp.Size(context.Background())).Should(Equal(entryCount))
	})

	It("should persist the map successfully created configs into the configmap", Label("fast"), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("hmp-3")
		maps := []string{"map1", "map2", "map3", "mapfail"}

		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey, 3)

		By("creating the map configs successfully")
		for i, mapp := range maps {
			m := hazelcastconfig.DefaultMap(types.NamespacedName{Name: mapp, Namespace: hazelcast.Namespace}, hazelcast.Name, labels)
			m.Spec.Eviction = &hazelcastcomv1alpha1.EvictionConfig{MaxSize: pointer.Int32Ptr(int32(i) * 100)}
			m.Spec.HazelcastResourceName = hazelcast.Name
			if mapp == "mapfail" {
				m.Spec.HazelcastResourceName = "failedHz"
			}
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			if mapp == "mapfail" {
				assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)
				continue
			}
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		}

		By("checking if the maps are in the map config")
		hzConfig := assertMapConfigsPersisted(hazelcast, "map1", "map2", "map3")
		for i, mapp := range maps {
			if mapp != "mapfail" {
				Expect(hzConfig.Hazelcast.Map[mapp].Eviction.Size).Should(Equal(int32(i) * 100))
			}
		}

		By("deleting map2")
		Expect(k8sClient.Delete(context.Background(),
			&hazelcastcomv1alpha1.Map{ObjectMeta: v1.ObjectMeta{Name: "map2", Namespace: hazelcast.Namespace}})).Should(Succeed())

		By("checking if map2 is not persisted in the configmap")
		_ = assertMapConfigsPersisted(hazelcast, "map1", "map3")
	})

	It("should persist Map Config with Indexes", Label("fast"), func() {
		setLabelAndCRName("hmp-4")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		m.Spec.Indexes = []hazelcastcomv1alpha1.IndexConfig{
			{
				Name:       "index-1",
				Type:       hazelcastcomv1alpha1.IndexTypeHash,
				Attributes: []string{"attribute1", "attribute2"},
				BitmapIndexOptions: &hazelcastcomv1alpha1.BitmapIndexOptionsConfig{
					UniqueKey:           "key",
					UniqueKeyTransition: hazelcastcomv1alpha1.UniqueKeyTransitionRAW,
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("checking if the map is in the configmap")
		hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)

		By("checking if the indexes are persisted")
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Name).Should(Equal("index-1"))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Type).Should(Equal(string(hazelcastcomv1alpha1.IndexTypeHash)))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].Attributes).Should(ConsistOf("attribute1", "attribute2"))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].BitmapIndexOptions.UniqueKey).Should(Equal("key"))
		Expect(hzConfig.Hazelcast.Map[m.Name].Indexes[0].BitmapIndexOptions.UniqueKeyTransformation).Should(Equal(string(hazelcastcomv1alpha1.UniqueKeyTransitionRAW)))
	})

	It("should continue persisting last applied Map Config in case of failure", Label("fast"), func() {
		setLabelAndCRName("hmp-5")
		hazelcast := hazelcastconfig.Default(hzLookupKey, ee, labels)
		CreateHazelcastCR(hazelcast)

		m := hazelcastconfig.DefaultMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		m = assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

		By("checking if the map config is persisted")
		hzConfig := assertMapConfigsPersisted(hazelcast, m.Name)
		mcfg := hzConfig.Hazelcast.Map[m.Name]

		By("failing to update the map config")
		m.Spec.BackupCount = pointer.Int32Ptr(4)
		Expect(k8sClient.Update(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapFailed)

		By("checking if the same map config is still there")
		// Should wait for Hazelcast reconciler to get triggered, we do not have a waiting mechanism for that.
		Sleep(5 * Second)
		hzConfig = assertMapConfigsPersisted(hazelcast, m.Name)
		newMcfg := hzConfig.Hazelcast.Map[m.Name]
		Expect(newMcfg).To(Equal(mcfg))

	})
})
