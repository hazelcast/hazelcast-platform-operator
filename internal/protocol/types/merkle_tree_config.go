package types

type MerkleTreeConfig struct {
	Enabled    bool  `xml:"enabled"`
	Depth      int32 `xml:"depth"`
	EnabledSet bool
}
