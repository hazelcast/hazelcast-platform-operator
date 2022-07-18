package types

// manual
type WanReplicationState = byte

// manual
const (
	WanReplicationStateReplicating = 0
	WanReplicationStatePaused      = 1
	WanReplicationStateStopped     = 2
)
