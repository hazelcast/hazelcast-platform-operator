package types

type ClusterMetadata struct {
	CurrentState  ClusterState
	MemberVersion string
	JetVersion    string
	ClusterTime   int64
}
