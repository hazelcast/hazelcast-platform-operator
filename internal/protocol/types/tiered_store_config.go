package types

type TieredStoreConfig struct {
	Enabled          bool
	MemoryTierConfig MemoryTierConfig
	DiskTierConfig   DiskTierConfig
}
