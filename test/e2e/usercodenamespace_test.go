package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	hzTypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast User Code Deployment", Group("user_code_namespace"), func() {
	localPort := strconv.Itoa(8800 + GinkgoParallelProcess())

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

	It("verify addition of entry listeners in Hazelcast map using UserCodeNamespace", Tag(Kind|Any), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("ucn-1")

		h := hazelcastconfig.Default(hzLookupKey, ee, labels)
		h.Spec.UserCodeNamespaces = &hazelcastcomv1alpha1.UserCodeNamespacesConfig{}
		CreateHazelcastCR(h)

		By("create UserCodeNamespace")
		ucns := hazelcastcomv1alpha1.UserCodeNamespaceSpec{
			HazelcastResourceName: hzLookupKey.Name,
			BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/entryListener",
			},
		}
		ucn := hazelcastconfig.UserCodeNamespace(ucns, mapLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), ucn)).Should(Succeed())
		assertUserCodeNamespaceStatus(ucn, hazelcastcomv1alpha1.UserCodeNamespaceSuccess)

		By("creating map with Map with entry listener")
		ms := hazelcastcomv1alpha1.MapSpec{
			DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
				HazelcastResourceName: hzLookupKey.Name,
				UserCodeNamespace:     ucn.Name,
			},
			EntryListeners: []hazelcastcomv1alpha1.EntryListenerConfiguration{
				{
					ClassName: "org.example.SampleEntryListener",
				},
			},
		}
		m := hazelcastconfig.Map(ms, mapLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		t := Now()

		By("port-forwarding to Hazelcast master pod")
		stopChan := portForwardPod(h.Name+"-0", h.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		By("filling the map with entries")
		entryCount := 5
		cl := newHazelcastClientPortForward(context.Background(), h, localPort)
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
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		var logEl []interface{}
		for _, e := range entries {
			logEl = append(logEl, fmt.Sprintf("EntryAdded, key: %s, value:%s", e.Key, e.Value))
		}
		test.EventuallyInLogsUnordered(logReader, 10*Second, logInterval).Should(ContainElements(logEl...))
	})

	It("verify added UCN works after cluster pause", Tag(Kind|Any), func() {
		if !ee {
			Skip("This test will only run in EE configuration")
		}
		setLabelAndCRName("ucn-1")

		h := hazelcastconfig.Default(hzLookupKey, ee, labels)
		h.Spec.UserCodeNamespaces = &hazelcastcomv1alpha1.UserCodeNamespacesConfig{}
		CreateHazelcastCR(h)

		By("create UserCodeNamespace")
		ucns := hazelcastcomv1alpha1.UserCodeNamespaceSpec{
			HazelcastResourceName: hzLookupKey.Name,
			BucketConfiguration: &hazelcastcomv1alpha1.BucketConfiguration{
				SecretName: "br-secret-gcp",
				BucketURI:  "gs://operator-user-code/entryListener",
			},
		}
		ucn := hazelcastconfig.UserCodeNamespace(ucns, mapLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), ucn)).Should(Succeed())
		assertUserCodeNamespaceStatus(ucn, hazelcastcomv1alpha1.UserCodeNamespaceSuccess)

		By("pause Hazelcast")
		UpdateHazelcastCR(h, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(0)
			return hazelcast
		})
		WaitForReplicaSize(h.Namespace, h.Name, 0)

		By("resume Hazelcast")
		UpdateHazelcastCR(h, func(hazelcast *hazelcastcomv1alpha1.Hazelcast) *hazelcastcomv1alpha1.Hazelcast {
			hazelcast.Spec.ClusterSize = pointer.Int32(3)
			return hazelcast
		})
		evaluateReadyMembers(hzLookupKey)

		By("creating map with Map with entry listener")
		ms := hazelcastcomv1alpha1.MapSpec{
			DataStructureSpec: hazelcastcomv1alpha1.DataStructureSpec{
				HazelcastResourceName: hzLookupKey.Name,
				UserCodeNamespace:     ucn.Name,
			},
			EntryListeners: []hazelcastcomv1alpha1.EntryListenerConfiguration{
				{
					ClassName: "org.example.SampleEntryListener",
				},
			},
		}
		m := hazelcastconfig.Map(ms, mapLookupKey, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		t := Now()

		By("port-forwarding to Hazelcast master pod")
		stopChan := portForwardPod(h.Name+"-0", h.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		By("filling the map with entries")
		entryCount := 5
		cl := newHazelcastClientPortForward(context.Background(), h, localPort)
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
		logReader := test.NewLogReader(logs)
		defer logReader.Close()
		var logEl []interface{}
		for _, e := range entries {
			logEl = append(logEl, fmt.Sprintf("EntryAdded, key: %s, value:%s", e.Key, e.Value))
		}
		test.EventuallyInLogsUnordered(logReader, 10*Second, logInterval).Should(ContainElements(logEl...))
	})
})
