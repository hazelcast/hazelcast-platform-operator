package types

type TieredStoreConfig struct {
	Enabled          bool             `xml:"enabled,attr"`
	MemoryTierConfig MemoryTierConfig `xml:"memory-tier"`
	DiskTierConfig   DiskTierConfig   `xml:"disk-tier"`
}
