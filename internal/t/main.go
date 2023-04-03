package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/hazelcast/hazelcast-go-client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

func main() {

	notAllKeys := []string{"A", "B", "C", "D", "E", "F", "G"}
	additionalKeys := []string{"H", "I", "G", "J"}
	var keys []string
	keys = append(keys, notAllKeys...)
	keys = append(keys, additionalKeys...)
	ticker := time.NewTicker(1 * time.Second)
	var m sync.Mutex
	store := make(map[string]string)
	for _, key := range notAllKeys {
		m.Lock()
		store[key] = key
		m.Unlock()
	}

	go func() {
		for _, key := range keys {
			m.Lock()
			println("Adding " + key)
			store[key] = key
			m.Unlock()
			time.Sleep(1 * time.Second)
		}
	}()

	for len(store) > 0 {
		select {
		case <-ticker.C:
			print("Tick... ")
			key := keys[rand.Intn(len(keys))]
			print(len(store))
			println(key)
			m.Lock()
			delete(store, key)
			m.Unlock()
		}
	}

	if 1 == 1 {
		return
	}
	ctx := context.Background()
	config := hazelcast.Config{}
	config.Cluster.Unisocket = true
	//config.Cluster.Network.Addresses = []string{"34.78.106.160:5701"}
	c, _ := hazelcast.StartNewClientWithConfig(ctx, config)
	ci := hazelcast.NewClientInternal(c)
	for _, m := range ci.OrderedMembers() {
		println(m.String())
	}

	md := types.JobMetaData{}
	md.JobName = "myjob"
	md.JarOnMember = true
	md.FileName = "/opt/hazelcast/userCode/bucket/jet-pipeline-1.0.2.jar"
	request := codec.EncodeJetUploadJobMetaDataRequest(md)
	_, err := ci.InvokeOnRandomTarget(ctx, request, nil)
	if err != nil {
		panic(err)
	}

	listRequest := codec.EncodeJetGetJobAndSqlSummaryListRequest()
	resp, err := ci.InvokeOnRandomTarget(ctx, listRequest, nil)
	if err != nil {
		panic(err)
	}

	listResponse := codec.DecodeJetGetJobAndSqlSummaryListResponse(resp)
	for _, jobSummary := range listResponse {
		println("The summary from list")
		println(fmt.Sprintf("%v", jobSummary))
	}
}
