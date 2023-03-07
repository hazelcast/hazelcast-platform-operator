package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"

	"github.com/hazelcast/hazelcast-go-client"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const valueLen = 8192

func main() {
	var address, clusterName, size, mapName string
	var wg sync.WaitGroup
	flag.StringVar(&address, "address", "localhost", "Pod address")
	flag.StringVar(&clusterName, "clusterName", "dev", "Cluster Name")
	flag.StringVar(&size, "size", "1024", "Desired map size")
	flag.StringVar(&mapName, "mapName", "map", "Map name")
	flag.Parse()

	ctx := context.Background()
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
	log.Printf("Successfully connected to '%s' and cluster '%s'.", address, clusterName)
	log.Printf("Starting to fill the map '%s' with entries.", mapName)

	m, err := client.GetMap(ctx, mapName)
	mapSizeInMb, err := strconv.ParseFloat(size, 64)
	entriesPerGoroutine := int(mapSizeInMb * 2)
	if err != nil {
		log.Fatal(err)
	}
	goroutineCount := 64
	log.Printf("Goroutine count: %d", goroutineCount)
	log.Printf("Entries per goroutine: %d", entriesPerGoroutine)

	for i := 1; i <= goroutineCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var r = rand.New(rand.NewSource(rand.Int63()))
			for j := 1; j <= entriesPerGoroutine; j++ {
				key := fmt.Sprintf("%s-%s-%s-%s", clusterName, randString(r, 5), randString(r, 5), randString(r, 5))
				value := randString(r, valueLen)
				mapInjector(ctx, m, key, value)
			}
		}()
	}
	wg.Wait()
	finalSize, _ := m.Size(ctx)
	log.Printf("Finished to fill the map with entries. Total entries were added %d. Current map size is %d ", entriesPerGoroutine*goroutineCount, finalSize)
}

func mapInjector(ctx context.Context, m *hazelcast.Map, key, value string) {
	log.Printf("Key: %s is putting into map.", key)
	_, err := m.Put(ctx, key, value)
	if err != nil {
		log.Fatal(err)
	}
}

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"
const charsetLen = len(charset)

func randString(r *rand.Rand, length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(charsetLen)]
	}
	return string(b)
}
