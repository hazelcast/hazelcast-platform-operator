package main

import (
	"context"
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"log"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/hazelcast/hazelcast-go-client"
)

func main() {
	var address string
	var size string
	var mapName string
	flag.StringVar(&address, "address", "localhost", "Pod address")
	flag.StringVar(&size, "size", "1", "Size of the map")
	flag.StringVar(&mapName, "mapName", "map", "Name of the map")
	flag.Parse()
	var wg sync.WaitGroup
	config := hazelcast.Config{}
	cc := &config.Cluster
	cc.Network.SetAddresses(address)
	ctx := context.Background()
	client, err := hazelcast.StartNewClientWithConfig(ctx, config)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successful connection!")
	fmt.Println("Starting to fill the map with entries.")
	longString := string(make([]byte, 8192))
	m, err := client.GetMap(ctx, mapName)
	m.Clear(ctx)
	mapSize, _ := strconv.ParseFloat(size, 64)
	if err != nil {
		panic(err)
	}
	n := int(math.Round(mapSize * 1310.72))
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			for j := 1; j <= n; j++ {
				key := fmt.Sprintf("%d-%d", i, j)
				mapInjector(ctx, m, key, longString)
			}
		}()
	}
	wg.Wait()
	WaitForMapSize(ctx, m, n*100, 5*time.Second, 20*time.Minute)
	fmt.Println("Finish to fill the map with entries.")
	fmt.Println(m.Size(ctx))
	err = client.Shutdown(ctx)
	if err != nil {
		panic(err)
	}
}

func mapInjector(ctx context.Context, m *hazelcast.Map, key string, longString string) {
	fmt.Printf("Key: %s is putting into map.\n", key)
	_, err := m.Put(ctx, key, longString)
	if err != nil {
		panic(err)
	}
}

func WaitForMapSize(ctx context.Context, m *hazelcast.Map, expectedSize int, interval time.Duration, timeout time.Duration) {
	var size int
	if err := wait.Poll(interval, timeout, func() (bool, error) {
		size, _ = m.Size(ctx)
		return size == expectedSize, nil
	}); err != nil {
		log.Fatalf("Error waiting for map size to reach expected number %v", size)
	}
}
