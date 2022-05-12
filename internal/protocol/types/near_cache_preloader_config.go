package types

type NearCachePreloaderConfig struct {
	Enabled                  bool
	Directory                string
	StoreInitialDelaySeconds int32
	StoreIntervalSeconds     int32
}
