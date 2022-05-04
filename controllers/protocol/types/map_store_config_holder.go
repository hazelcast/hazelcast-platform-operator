package types

import iserialization "github.com/hazelcast/hazelcast-go-client"

type MapStoreConfigHolder struct {
	Enabled               bool
	WriteCoalescing       bool
	WriteDelaySeconds     int32
	WriteBatchSize        int32
	ClassName             string
	Implementation        iserialization.Data
	FactoryClassName      string
	FactoryImplementation iserialization.Data
	Properties            map[string]string
	InitialLoadMode       string
}
