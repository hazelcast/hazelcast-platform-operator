package types

type AddExecutorInput struct {
	Name              string
	PoolSize          int32
	QueueCapacity     int32
	StatisticsEnabled bool
	// nullable
	SplitBrainProtectionName string
}

type AddDurableExecutorInput struct {
	Name              string
	PoolSize          int32
	Durability        int32
	Capacity          int32
	StatisticsEnabled bool
	// nullable
	SplitBrainProtectionName string
}

type AddScheduledExecutorInput struct {
	Name              string
	PoolSize          int32
	Durability        int32
	Capacity          int32
	CapacityPolicy    string
	StatisticsEnabled bool
	MergePolicy       string
	MergeBatchSize    int32
	// nullable
	SplitBrainProtectionName string
}

func DefaultAddExecutorServiceInput() *AddExecutorInput {
	return &AddExecutorInput{
		StatisticsEnabled: true,
	}
}

func DefaultAddDurableExecutorServiceInput() *AddDurableExecutorInput {
	return &AddDurableExecutorInput{
		StatisticsEnabled: true,
	}
}

func DefaultAddScheduledExecutorServiceInput() *AddScheduledExecutorInput {
	return &AddScheduledExecutorInput{
		StatisticsEnabled: true,
		MergePolicy:       "PutIfAbsentMergePolicy",
		MergeBatchSize:    100,
	}
}
