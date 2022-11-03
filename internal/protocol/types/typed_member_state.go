package types

import (
	"time"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

type TimedMemberStateWrapper struct {
	TimedMemberState TimedMemberState `json:"timedMemberState"`
}

type TimedMemberState struct {
	MemberState          MemberState          `json:"memberState"`
	MemberPartitionState MemberPartitionState `json:"memberPartitionState"`
	Master               bool                 `json:"master"`
}

type MemberState struct {
	Address                 string                  `json:"address"`
	Uuid                    string                  `json:"uuid"`
	Name                    string                  `json:"name"`
	NodeState               NodeState               `json:"nodeState"`
	HotRestartState         HotRestartState         `json:"hotRestartState"`
	ClusterHotRestartStatus ClusterHotRestartStatus `json:"clusterHotRestartStatus"`
}

type NodeState struct {
	State         string `json:"nodeState"`
	MemberVersion string `json:"memberVersion"`
}

type MemberPartitionState struct {
	Partitions []int32 `json:"partitions"`
}

type HotRestartState struct {
	BackupTaskState     string `json:"backupTaskState"`
	BackupTaskCompleted int32  `json:"backupTaskCompleted"`
	BackupTaskTotal     int32  `json:"backupTaskTotal"`
	IsHotBackupEnabled  bool   `json:"isHotBackupEnabled"`
	BackupDirectory     string `json:"backupDirectory"`
}

type ClusterHotRestartStatus struct {
	HotRestartStatus              string `json:"hotRestartStatus"`
	RemainingValidationTimeMillis int64  `json:"remainingValidationTimeMillis"`
	RemainingDataLoadTimeMillis   int64  `json:"remainingDataLoadTimeMillis"`
}

func (c ClusterHotRestartStatus) RemainingValidationTimeSec() int64 {
	return int64((time.Duration(c.RemainingValidationTimeMillis) * time.Millisecond).Seconds())
}

func (c ClusterHotRestartStatus) RemainingDataLoadTimeSec() int64 {
	return int64((time.Duration(c.RemainingDataLoadTimeMillis) * time.Millisecond).Seconds())
}

func (c ClusterHotRestartStatus) RestoreState() hazelcastv1alpha1.RestoreState {
	switch c.HotRestartStatus {
	case "SUCCEEDED":
		return hazelcastv1alpha1.RestoreSucceeded
	case "IN_PROGRESS":
		return hazelcastv1alpha1.RestoreInProgress
	case "FAILED":
		return hazelcastv1alpha1.RestoreFailed
	default:
		return hazelcastv1alpha1.RestoreUnknown
	}
}
