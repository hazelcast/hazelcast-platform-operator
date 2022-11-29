package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	hzclienttypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

func FillTheMapDataPortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort, mapName string, entryCount int) {
	By(fmt.Sprintf("filling the '%s' map with '%d' entries using '%s' lookup name and '%s' namespace", mapName, entryCount, hz.Name, hz.Namespace), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		m, err := cl.GetMap(ctx, mapName)
		Expect(err).ToNot(HaveOccurred())
		initMapSize, err := m.Size(ctx)
		Expect(err).ToNot(HaveOccurred())
		entries := make([]hzclienttypes.Entry, 0, entryCount)
		for i := initMapSize; i < initMapSize+entryCount; i++ {
			entries = append(entries, hzclienttypes.NewEntry(strconv.Itoa(i), strconv.Itoa(i)))
		}
		err = m.PutAll(ctx, entries...)
		Expect(err).ToNot(HaveOccurred())
		mapSize, err := m.Size(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(mapSize).To(Equal(initMapSize + entryCount))
	})
}

func WaitForMapSizePortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort, mapName string, mapSize int, timeout Duration) {
	By(fmt.Sprintf("waiting the '%s' map to be of size '%d' using lookup name '%s'", mapName, mapSize, hz.Name), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		if timeout == 0 {
			timeout = 10 * Minute
		}

		Eventually(func() (int, error) {
			hzMap, err := cl.GetMap(ctx, mapName)
			if err != nil {
				return -1, err
			}
			return hzMap.Size(ctx)
		}, timeout, 10*Second).Should(Equal(mapSize))
	})
}

func MemberConfigPortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort string) string {
	cfg := ""
	By(fmt.Sprintf("Getting the member config with lookup name '%s'", hz.Name), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		cfg = getMemberConfig(context.Background(), cl)
	})
	return cfg
}

func MapConfigPortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort, mapName string) codecTypes.MapConfig {
	cfg := codecTypes.MapConfig{}
	By(fmt.Sprintf("Getting the member config with lookup name '%s'", hz.Name), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		cfg = getMapConfig(context.Background(), cl, mapName)
	})
	return cfg
}
