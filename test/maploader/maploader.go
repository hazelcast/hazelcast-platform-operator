package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"sync"

	"github.com/hazelcast/hazelcast-go-client"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"

func main() {
	var address, clusterName, size, mapName string
	var wg sync.WaitGroup
	flag.StringVar(&address, "address", "localhost", "Pod address")
	flag.StringVar(&clusterName, "clusterName", "dev", "Cluster Name")
	flag.StringVar(&size, "size", "1", "Desired map size")
	flag.StringVar(&mapName, "mapName", "map", "Map name")
	flag.Parse()

	ctx := context.Background()
	value := string(make([]byte, 8192))
	config := hazelcast.Config{}
	config.Cluster.Network.SetAddresses(address)
	config.Cluster.Name = clusterName
	config.Cluster.Discovery.UsePublicIP = true
	client, err := hazelcast.StartNewClientWithConfig(ctx, config)
	defer func() {
		err := client.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}()
	fmt.Printf("Successfully connected to '%s' and cluster '%s'!\nStarting to fill the map '%s' with entries.", address, clusterName, mapName)
	m, err := client.GetMap(ctx, mapName)
	mapSize, err := strconv.ParseFloat(size, 64)
	entriesPerThread := int(mapSize * math.Round(1310.72))
	if err != nil {
		log.Fatal(err)
	}
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 1; j <= entriesPerThread; j++ {
				key := fmt.Sprintf("%s-%s-%s-%s", clusterName, randString(5), randString(5), randString(5))
				mapInjector(ctx, m, key, value)
			}
		}()
	}
	wg.Wait()
	finalSize, _ := m.Size(ctx)
	fmt.Printf("Finished to fill the map with entries. Total entries were added %d. Current map size is %d ", entriesPerThread*100, finalSize)

}

func mapInjector(ctx context.Context, m *hazelcast.Map, key, value string) {
	fmt.Printf("Key: %s is putting into map.\n", key)
	_, err := m.Put(ctx, key, value)
	if err != nil {
		log.Fatal(err)
	}
}

func randString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
