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

package codec

import (
	proto "github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	DynamicConfigAddCacheConfigCodecRequestMessageType  = int32(0x1B0E00)
	DynamicConfigAddCacheConfigCodecResponseMessageType = int32(0x1B0E01)

	DynamicConfigAddCacheConfigCodecRequestStatisticsEnabledOffset                 = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestManagementEnabledOffset                 = DynamicConfigAddCacheConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestReadThroughOffset                       = DynamicConfigAddCacheConfigCodecRequestManagementEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestWriteThroughOffset                      = DynamicConfigAddCacheConfigCodecRequestReadThroughOffset + proto.BooleanSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestBackupCountOffset                       = DynamicConfigAddCacheConfigCodecRequestWriteThroughOffset + proto.BooleanSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestAsyncBackupCountOffset                  = DynamicConfigAddCacheConfigCodecRequestBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestMergeBatchSizeOffset                    = DynamicConfigAddCacheConfigCodecRequestAsyncBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestDisablePerEntryInvalidationEventsOffset = DynamicConfigAddCacheConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddCacheConfigCodecRequestInitialFrameSize                        = DynamicConfigAddCacheConfigCodecRequestDisablePerEntryInvalidationEventsOffset + proto.BooleanSizeInBytes
)

// Adds a new cache configuration to a running cluster.
// If a cache configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddCacheConfigRequest(c *types.CacheConfigInput) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddCacheConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestStatisticsEnabledOffset, c.StatisticsEnabled)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestManagementEnabledOffset, c.ManagementEnabled)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestReadThroughOffset, c.ReadThrough)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestWriteThroughOffset, c.WriteThrough)
	EncodeInt(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestBackupCountOffset, c.BackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestAsyncBackupCountOffset, c.AsyncBackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestMergeBatchSizeOffset, c.MergeBatchSize)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddCacheConfigCodecRequestDisablePerEntryInvalidationEventsOffset, c.DisablePerEntryInvalidationEvents)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddCacheConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, c.Name)
	EncodeNullableForString(clientMessage, c.KeyType)
	EncodeNullableForString(clientMessage, c.ValueType)
	EncodeNullableForString(clientMessage, c.CacheLoaderFactory)
	EncodeNullableForString(clientMessage, c.CacheWriterFactory)
	EncodeNullableForString(clientMessage, c.CacheLoader)
	EncodeNullableForString(clientMessage, c.CacheWriter)
	EncodeString(clientMessage, string(c.InMemoryFormat))
	EncodeNullableForString(clientMessage, c.SplitBrainProtectionName)
	EncodeNullableForString(clientMessage, c.MergePolicy)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.PartitionLostListenerConfigs)
	EncodeNullableForString(clientMessage, c.ExpiryPolicyFactoryClassName)
	EncodeNullableForEncodeTimedExpiryPolicyFactoryConfig(clientMessage, c.TimedExpiryPolicyFactoryConfig)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.CacheEntryListeners)
	EncodeNullableForEvictionConfigHolder(clientMessage, c.EvictionConfig)
	EncodeNullableForWanReplicationRef(clientMessage, c.WanReplicationRef)
	EncodeNullableForEventJournalConfig(clientMessage, c.EventJournalConfig)
	EncodeNullableForHotRestartConfig(clientMessage, c.HotRestartConfig)
	EncodeNullableForMerkleTreeConfig(clientMessage, c.MerkleTreeConfig)
	EncodeDataPersistenceConfig(clientMessage, c.DataPersistenceConfig)

	return clientMessage
}
