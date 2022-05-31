package types

import iserialization "github.com/hazelcast/hazelcast-go-client"

type ListenerConfigHolder struct {
	ListenerType           int32
	ListenerImplementation iserialization.Data
	ClassName              string
	IncludeValue           bool
	Local                  bool
}
