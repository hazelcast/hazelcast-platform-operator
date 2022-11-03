package types

import n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"

type QueueConfigs struct {
	Queues []QueueConfigInput `xml:"queue"`
}

type QueueConfigInput struct {
	Name                        string `xml:"name,attr"`
	ListenerConfigs             []ListenerConfigHolder
	BackupCount                 int32 `xml:"backup-count"`
	AsyncBackupCount            int32 `xml:"async-backup-count"`
	MaxSize                     int32 `xml:"max-size"`
	EmptyQueueTtl               int32 `xml:"empty-queue-ttl"`
	StatisticsEnabled           bool  `xml:"statistics-enabled"`
	SplitBrainProtectionName    string
	QueueStoreConfig            QueueStoreConfigHolder
	MergePolicy                 string
	MergeBatchSize              int32
	PriorityComparatorClassName string `xml:"priority-comparator-class-name"`
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
