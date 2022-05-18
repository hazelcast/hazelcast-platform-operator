package types

type MapConfig struct {
	InMemoryFormat    int32
	BackupCount       int32
	AsyncBackupCount  int32
	TimeToLiveSeconds int32
	MaxIdleSeconds    int32
	MaxSize           int32
	MaxSizePolicy     int32
	ReadBackupData    bool
	EvictionPolicy    int32
	MergePolicy       string
	Indexes           []IndexConfig
}
