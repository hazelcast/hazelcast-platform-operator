/*
* Copyright (c) 2008-2023, Hazelcast, Inc. All Rights Reserved.
*
* Licensed under the Apache License, Version 2.0 (the "License")
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package types

import "github.com/hazelcast/hazelcast-go-client/types"

type TerminateMode int32

const (
	RestartGracefully TerminateMode = iota
	RestartForcefully
	SuspendGracefully
	SuspendForcefully
	CancelGracefully
	CancelForcefully
)

type JetTerminateJob struct {
	JobId               int64
	TerminateMode       TerminateMode
	LightJobCoordinator types.UUID
}

type JobAndSqlSummary struct {
	LightJob        bool
	JobId           int64
	ExecutionId     int64
	NameOrId        string
	Status          int32
	SubmissionTime  int64
	CompletionTime  int64
	FailureText     string
	SqlSummary      SqlSummary
	SuspensionCause string
}

type SqlSummary struct {
	Query     string
	Unbounded bool
}

type JobMetaData struct {
	SessionId     types.UUID
	JarOnMember   bool
	FileName      string
	Sha256Hex     string
	SnapshotName  string
	JobName       string
	MainClass     string
	JobParameters []string
}

func DefaultExistingJarJobMetaData(jobName string, jarName string) JobMetaData {
	return JobMetaData{
		JobName:     jobName,
		FileName:    jarName,
		JarOnMember: true,
	}
}
