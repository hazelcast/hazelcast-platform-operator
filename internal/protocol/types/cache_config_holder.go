/*
* Copyright (c) 2008-2022, Hazelcast, Inc. All Rights Reserved.
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

import iserialization "github.com/hazelcast/hazelcast-go-client"

type CacheConfigHolder struct {
	Name                              string
	ManagerPrefix                     string
	UriString                         string
	BackupCount                       int32
	AsyncBackupCount                  int32
	InMemoryFormat                    string
	EvictionConfigHolder              EvictionConfigHolder
	WanReplicationRef                 WanReplicationRef
	KeyClassName                      string
	ValueClassName                    string
	CacheLoaderFactory                iserialization.Data
	CacheWriterFactory                iserialization.Data
	ExpiryPolicyFactory               iserialization.Data
	ReadThrough                       bool
	WriteThrough                      bool
	StoreByValue                      bool
	ManagementEnabled                 bool
	StatisticsEnabled                 bool
	HotRestartConfig                  HotRestartConfig
	EventJournalConfig                EventJournalConfig
	SplitBrainProtectionName          string
	ListenerConfigurations            []iserialization.Data
	MergePolicyConfig                 MergePolicyConfig
	DisablePerEntryInvalidationEvents bool
	CachePartitionLostListenerConfigs []ListenerConfigHolder
	MerkleTreeConfig                  MerkleTreeConfig
	DataPersistenceConfig             DataPersistenceConfig
}
