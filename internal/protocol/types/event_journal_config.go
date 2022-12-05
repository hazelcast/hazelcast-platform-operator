package types

type EventJournalConfig struct {
	Enabled           bool
	Capacity          int32
	TimeToLiveSeconds int32
}
