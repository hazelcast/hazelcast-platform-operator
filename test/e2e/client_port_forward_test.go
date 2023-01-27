package e2e

import (
	"context"
	"fmt"
	"strconv"
	. "time"

	hzClient "github.com/hazelcast/hazelcast-go-client"
	hzclienttypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

func fillTheMapDataPortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort, mapName string, entryCount int) {
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

func waitForMapSizePortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort, mapName string, mapSize int, timeout Duration) {
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

func memberConfigPortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort string) string {
	cfg := ""
	By(fmt.Sprintf("Getting the member config with lookup name '%s'", hz.Name), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		cfg = getMemberConfig(ctx, cl)
	})
	return cfg
}

func mapConfigPortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort, mapName string) codecTypes.MapConfig {
	cfg := codecTypes.MapConfig{}
	By(fmt.Sprintf("Getting the map config with lookup name '%s'", hz.Name), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		cfg = getMapConfig(ctx, cl, mapName)
	})
	return cfg
}

func assertClusterStatePortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort string, state codecTypes.ClusterState) {
	By("waiting for Cluster state", func() {
		Eventually(func() codecTypes.ClusterState {
			return clusterStatePortForward(ctx, hz, localPort)
		}, 30*Second, interval).Should(Equal(state))
	})
}

func clusterStatePortForward(ctx context.Context, hz *hazelcastcomv1alpha1.Hazelcast, localPort string) codecTypes.ClusterState {
	state := codecTypes.ClusterState(-1)
	By(fmt.Sprintf("Getting the cluster state with lookup name '%s'", hz.Name), func() {
		stopChan := portForwardPod(hz.Name+"-0", hz.Namespace, localPort+":5701")
		defer closeChannel(stopChan)

		cl := newHazelcastClientPortForward(ctx, hz, localPort)
		defer func() {
			err := cl.Shutdown(ctx)
			Expect(err).To(BeNil())
		}()

		req := codec.EncodeMCGetClusterMetadataRequest()
		ci := hzClient.NewClientInternal(cl)
		resp, err := ci.InvokeOnRandomTarget(ctx, req, nil)
		Expect(err).To(BeNil())
		metadata := codec.DecodeMCGetClusterMetadataResponse(resp)
		state = metadata.CurrentState
	})
	return state
}
