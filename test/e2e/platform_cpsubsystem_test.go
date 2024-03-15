package e2e

import (
	"context"
	"strconv"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("CP Subsystem", func() {
	localPort := strconv.Itoa(8900 + GinkgoParallelProcess())

	createCPGroup := func(ctx context.Context, cli *hzClient.ClientInternal) types.RaftGroupId {
		grResp, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPGroupCreateCPGroupRequest("new-group"), nil)
		Expect(err).To(BeNil())
		return codec.DecodeCPGroupCreateCPGroupResponse(grResp)
	}

	writeToCPMap := func(ctx context.Context, cli *hzClient.ClientInternal, mapName, key, value string, rg types.RaftGroupId) {
		keyD, _ := cli.EncodeData(key)
		valueD, _ := cli.EncodeData(value)
		_, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPMapPutRequest(rg, mapName, keyD, valueD), nil)
		Expect(err).To(BeNil())
	}

	readFromCPMap := func(ctx context.Context, cli *hzClient.ClientInternal, mapName, key, expectedValue string, rg types.RaftGroupId) {
		keyD, _ := cli.EncodeData(key)
		r, err := cli.InvokeOnRandomTarget(ctx, codec.EncodeCPMapGetRequest(rg, mapName, keyD), nil)
		Expect(err).To(BeNil())
		response, err := cli.DecodeData(codec.DecodeCPMapGetResponse(r))
		Expect(err).To(BeNil())
		Expect(response).To(Equal(expectedValue))
	}

	validateCPMap := func(ctx context.Context, cli *hzClient.ClientInternal, mapName, key, value string) {
		rg := createCPGroup(ctx, cli)
		writeToCPMap(ctx, cli, mapName, key, value, rg)
		readFromCPMap(ctx, cli, mapName, key, value, rg)
	}

	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	DescribeTable("should store data in CP Map", Tag(EE|AnyCloud), func(hazelcastSpec hazelcastcomv1alpha1.HazelcastSpec) {
		setLabelAndCRName("cp-1")
		ctx := context.Background()
		cpMapName := "my-map"

		hazelcast := &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      hzLookupKey.Name,
				Namespace: hzLookupKey.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastSpec,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		clientHz := GetHzClient(ctx, hzLookupKey, true)
		cli := hzClient.NewClientInternal(clientHz)

		validateCPMap(ctx, cli, cpMapName, randString(5), randString(5))
	},
		Entry("with CP Subsystem PVC", hazelcastconfig.HazelcastCPSubsystem(3)),
		Entry("with Persistence PVC", hazelcastconfig.HazelcastCPSubsystemPersistence(3)),
	)

	DescribeTable("should store data in CP Map with cluster pause", Tag(EE|AnyCloud), func(hazelcastSpec hazelcastcomv1alpha1.HazelcastSpec) {
		setLabelAndCRName("cp-2")
		ctx := context.Background()
		cpMapName := "my-map"

		hazelcast := &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      hzLookupKey.Name,
				Namespace: hzLookupKey.Namespace,
				Labels:    labels,
			},
			Spec: hazelcastSpec,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		clientHz := GetHzClient(ctx, hzLookupKey, true)
		cli := hzClient.NewClientInternal(clientHz)

		rg := createCPGroup(ctx, cli)

		key := randString(5)
		value := randString(5)
		writeToCPMap(ctx, cli, cpMapName, key, value, rg)

		By("pause Hazelcast")
		UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(0)
			return hazelcast
		})
		WaitForReplicaSize(hazelcast.Namespace, hazelcast.Name, 0)

		By("resume Hazelcast")
		UpdateHazelcastCR(hazelcast, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(3)
			return hazelcast
		})
		evaluateReadyMembers(hzLookupKey)

		readFromCPMap(ctx, cli, cpMapName, key, value, rg)
	},
		Entry("with CP Subsystem PVC", hazelcastconfig.HazelcastCPSubsystem(3)),
		Entry("with Persistence PVC", hazelcastconfig.HazelcastCPSubsystemPersistence(3)),
	)

	It("should start CP with Persistence and different PVCs", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("cp-3")
		ctx := context.Background()
		cpMapName := "my-map"

		spec := hazelcastconfig.HazelcastCPSubsystemPersistence(3)
		spec.CPSubsystem.PVC = &hazelcastcomv1alpha1.PvcConfiguration{
			AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			RequestStorage: &[]resource.Quantity{resource.MustParse("2Gi")}[0],
		}
		hazelcast := &hazelcastcomv1alpha1.Hazelcast{
			ObjectMeta: v1.ObjectMeta{
				Name:      hzLookupKey.Name,
				Namespace: hzLookupKey.Namespace,
				Labels:    labels,
			},
			Spec: spec,
		}
		hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}

		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		clientHz := GetHzClient(ctx, hzLookupKey, true)
		cli := hzClient.NewClientInternal(clientHz)

		validateCPMap(ctx, cli, cpMapName, randString(5), randString(5))
	})

	It("Should work on cluster restored from HotBackup", Tag(EE|AnyCloud), func() {
		setLabelAndCRName("cp-3")
		ctx := context.Background()
		initialCluster := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, 3, labels)
		CreateHazelcastCR(initialCluster)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.PersistedMap(mapLookupKey, initialCluster.Name, labels)
		Expect(k8sClient.Create(ctx, m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		fillTheMapDataPortForward(ctx, initialCluster, localPort, m.MapName(), 10)

		By("triggering backup")
		hotBackup := hazelcastconfig.HotBackup(hbLookupKey, initialCluster.Name, labels)
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		By("removing Hazelcast CR")
		RemoveHazelcastCR(initialCluster)

		By("creating cluster from backup")
		restoredHz := hazelcastconfig.HazelcastRestore(initialCluster, restoreConfig(hotBackup, false))
		restoredHz.Spec.CPSubsystem = hazelcastconfig.HazelcastCPSubsystemPersistence(3).CPSubsystem
		restoredHz.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
			Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
			DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
			MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
		}
		CreateHazelcastCR(restoredHz)
		evaluateReadyMembers(hzLookupKey)
		waitForMapSizePortForward(context.Background(), restoredHz, localPort, m.MapName(), 10, 1*Minute)

		cpMapName := "my-cp-map"
		clientHz := GetHzClient(ctx, hzLookupKey, true)
		cli := hzClient.NewClientInternal(clientHz)
		validateCPMap(ctx, cli, cpMapName, randString(5), randString(5))
	})

}, Label("cp_subsystem"))
