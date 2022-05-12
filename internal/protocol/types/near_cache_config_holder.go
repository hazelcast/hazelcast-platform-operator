package types

type NearCacheConfigHolder struct {
	Name                 string
	InMemoryFormat       string
	SerializeKeys        bool
	InvalidateOnChange   bool
	TimeToLiveSeconds    int32
	MaxIdleSeconds       int32
	EvictionConfigHolder EvictionConfigHolder
	CacheLocalEntries    bool
	LocalUpdatePolicy    string
	PreloaderConfig      NearCachePreloaderConfig
}
