package types

import iserialization "github.com/hazelcast/hazelcast-go-client"

type PredicateConfigHolder struct {
	ClassName      string
	Sql            string
	Implementation iserialization.Data
}
