package types

type QueryCacheConfigHolder struct {
	BatchSize             int32
	BufferSize            int32
	DelaySeconds          int32
	IncludeValue          bool
	Populate              bool
	Coalesce              bool
	InMemoryFormat        string
	Name                  string
	PredicateConfigHolder PredicateConfigHolder
	EvictionConfigHolder  EvictionConfigHolder
	ListenerConfigs       []ListenerConfigHolder
	IndexConfigs          []IndexConfig
}
