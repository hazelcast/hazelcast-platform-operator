package types

type WanSyncType int8

const (
	AllMaps WanSyncType = iota
	SingleMap
)

type WanSyncRef struct {
	WanReplicationName string
	WanPublisherId     string
	Type               WanSyncType
	MapName            string
}
