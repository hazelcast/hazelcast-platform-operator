package types

import iserialization "github.com/hazelcast/hazelcast-go-client"

type EvictionConfigHolder struct {
	Size                int32
	MaxSizePolicy       string
	EvictionPolicy      string
	ComparatorClassName string
	Comparator          iserialization.Data
}
