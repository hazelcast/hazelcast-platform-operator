package main

import (
	"context"
	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

func main() {
	client, _ := hazelcast.StartNewClient(context.Background())
	ci := hazelcast.NewClientInternal(client)

	//epf, _ := ci.EncodeData("com.hazelcast.examples.declarative.ExpiryPolicyFactory")
	//createConfigRequest := codec.EncodeCacheCreateConfigRequest(types.CacheConfigHolder{
	//	Name:             "cache",
	//	ManagerPrefix:    "my-",
	//	UriString:        "",
	//	BackupCount:      1,
	//	AsyncBackupCount: 0,
	//	InMemoryFormat:   "BINARY",
	//	EvictionConfigHolder: types.EvictionConfigHolder{
	//		Size:           10000,
	//		MaxSizePolicy:  "ENTRY_COUNT",
	//		EvictionPolicy: "LRU",
	//	},
	//	WanReplicationRef:        types.WanReplicationRef{},
	//	KeyClassName:             "",
	//	ValueClassName:           "",
	//	CacheLoaderFactory:       nil,
	//	CacheWriterFactory:       nil,
	//	ExpiryPolicyFactory:      epf,
	//	ReadThrough:              false,
	//	WriteThrough:             false,
	//	StoreByValue:             false,
	//	ManagementEnabled:        false,
	//	StatisticsEnabled:        false,
	//	HotRestartConfig:         types.HotRestartConfig{IsDefined: true},
	//	EventJournalConfig:       types.EventJournalConfig{IsDefined: true, Capacity: 1},
	//	SplitBrainProtectionName: "",
	//	ListenerConfigurations:   nil,
	//	MergePolicyConfig: types.MergePolicyConfig{
	//		Policy:    naming.DefaultMergePolicyClassName,
	//		BatchSize: naming.DefaultCacheMergeBatchSize,
	//	},
	//	DisablePerEntryInvalidationEvents: false,
	//	CachePartitionLostListenerConfigs: nil,
	//	MerkleTreeConfig:                  types.MerkleTreeConfig{IsDefined: true, Depth: 2},
	//	DataPersistenceConfig:             types.DataPersistenceConfig{},
	//}, true)
	input := types.DefaultCacheConfigInput()
	input.Name = "my-cache"
	input.ExpiryPolicyFactoryClassName = "javax.cache.expiry.ExpiryPolicy"
	createConfigRequest := codec.EncodeDynamicConfigAddCacheConfigRequest(input)
	respose, err := ci.InvokeOnRandomTarget(context.Background(), createConfigRequest, nil)
	if err != nil {
		panic(err)
	}
	//holder := codec.DecodeCacheConfigHolder(respose.FrameIterator())
	//fmt.Println(holder)
	key, err := ci.EncodeData("mykey")
	if err != nil {
		panic(err)
	}
	value, err := ci.EncodeData("myvalue")
	if err != nil {
		panic(err)
	}
	request := codec.EncodeCachePutRequest("my-cache", key, value, nil, false, 42)
	//for _, mi := range ci.OrderedMembers() {
	//	respose, err = ci.InvokeOnMember(context.Background(), request, mi.UUID, nil)
	//	if err != nil {
	//		panic(err)
	//	}
	//	println(respose)
	//}
	//respose, err = ci.InvokeOnRandomTarget(context.Background(), request, nil)
	respose, err = ci.InvokeOnKey(context.Background(), request, key, nil)
	if err != nil {
		panic(err)
	}
	println(respose)
}
