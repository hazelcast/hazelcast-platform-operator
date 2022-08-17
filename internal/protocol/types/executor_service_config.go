package types

type ExecutorServices struct {
	Basic     []ExecutorServiceConfig          `xml:"executor-service"`
	Durable   []DurableExecutorServiceConfig   `xml:"durable-executor-service"`
	Scheduled []ScheduledExecutorServiceConfig `xml:"scheduled-executor-service"`
}

type ExecutorServiceConfig struct {
	Name              string `xml:"name,attr"`
	PoolSize          int32  `xml:"pool-size"`
	QueueCapacity     int32  `xml:"queue-capacity"`
	StatisticsEnabled bool
	// nullable
	SplitBrainProtectionName string
}

type DurableExecutorServiceConfig struct {
	Name              string `xml:"name,attr"`
	PoolSize          int32  `xml:"pool-size"`
	Durability        int32  `xml:"durability"`
	Capacity          int32  `xml:"capacity"`
	StatisticsEnabled bool
	// nullable
	SplitBrainProtectionName string
}

type ScheduledExecutorServiceConfig struct {
	Name              string `xml:"name,attr"`
	PoolSize          int32  `xml:"pool-size"`
	Durability        int32  `xml:"durability"`
	Capacity          int32  `xml:"capacity"`
	CapacityPolicy    string `xml:"capacity-policy"`
	StatisticsEnabled bool
	MergePolicy       string
	MergeBatchSize    int32
	// nullable
	SplitBrainProtectionName string
}

func DefaultAddExecutorServiceInput() *ExecutorServiceConfig {
	return &ExecutorServiceConfig{
		StatisticsEnabled: true,
	}
}

func DefaultAddDurableExecutorServiceInput() *DurableExecutorServiceConfig {
	return &DurableExecutorServiceConfig{
		StatisticsEnabled: true,
	}
}

func DefaultAddScheduledExecutorServiceInput() *ScheduledExecutorServiceConfig {
	return &ScheduledExecutorServiceConfig{
		StatisticsEnabled: true,
		MergePolicy:       "PutIfAbsentMergePolicy",
		MergeBatchSize:    100,
	}
}
