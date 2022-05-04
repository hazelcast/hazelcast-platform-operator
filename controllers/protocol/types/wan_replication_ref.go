package types

type WanReplicationRef struct {
	Name                 string
	MergePolicyClassName string
	Filters              []string
	RepublishingEnabled  bool
}
