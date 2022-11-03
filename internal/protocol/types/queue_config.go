package types

import n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"

type QueueConfigInput struct {
	Name                        string
	ListenerConfigs             []ListenerConfigHolder
	BackupCount                 int32
	AsyncBackupCount            int32
	MaxSize                     int32
	EmptyQueueTtl               int32
	StatisticsEnabled           bool
	SplitBrainProtectionName    string
	QueueStoreConfig            QueueStoreConfigHolder
	MergePolicy                 string
	MergeBatchSize              int32
	PriorityComparatorClassName string
}

// Default values are explicitly written for all fields that are not nullable
// even though most are the same with the default values in Go.
func DefaultQueueConfigInput() *QueueConfigInput {
	return &QueueConfigInput{
		BackupCount:       n.DefaultQueueBackupCount,
		AsyncBackupCount:  n.DefaultQueueAsyncBackupCount,
		MaxSize:           n.DefaultQueueMaxSize,
		EmptyQueueTtl:     n.DefaultQueueEmptyQueueTtl,
		StatisticsEnabled: n.DefaultQueueStatisticsEnabled,
		MergePolicy:       n.DefaultQueueMergePolicy,
		MergeBatchSize:    n.DefaultQueueMergeBatchSize,
	}
}
