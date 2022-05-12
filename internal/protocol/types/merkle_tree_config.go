package types

type MerkleTreeConfig struct {
	// IsDefined is an interim solution. Although HotRestartConfig is nullable, core side returns an error.
	// The reason for not using pointers is to using the heap at least as possible, it is a decision client team took.
	IsDefined  bool
	Enabled    bool
	Depth      int32
	EnabledSet bool
}
