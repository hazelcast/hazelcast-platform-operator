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

	types "github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
)

const (
	DynamicConfigAddMapConfigCodecRequestMessageType  = int32(0x1B0C00)
	DynamicConfigAddMapConfigCodecResponseMessageType = int32(0x1B0C01)

	DynamicConfigAddMapConfigCodecRequestBackupCountOffset          = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestAsyncBackupCountOffset     = DynamicConfigAddMapConfigCodecRequestBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestTimeToLiveSecondsOffset    = DynamicConfigAddMapConfigCodecRequestAsyncBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestMaxIdleSecondsOffset       = DynamicConfigAddMapConfigCodecRequestTimeToLiveSecondsOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestReadBackupDataOffset       = DynamicConfigAddMapConfigCodecRequestMaxIdleSecondsOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestMergeBatchSizeOffset       = DynamicConfigAddMapConfigCodecRequestReadBackupDataOffset + proto.BooleanSizeInBytes
	DynamicConfigAddMapConfigCodecRequestStatisticsEnabledOffset    = DynamicConfigAddMapConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestMetadataPolicyOffset       = DynamicConfigAddMapConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddMapConfigCodecRequestPerEntryStatsEnabledOffset = DynamicConfigAddMapConfigCodecRequestMetadataPolicyOffset + proto.IntSizeInBytes
	DynamicConfigAddMapConfigCodecRequestInitialFrameSize           = DynamicConfigAddMapConfigCodecRequestPerEntryStatsEnabledOffset + proto.BooleanSizeInBytes
)

// Adds a new map configuration to a running cluster.
// If a map configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddMapConfigRequest(c *types.AddMapConfigInput) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestBackupCountOffset, c.BackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestAsyncBackupCountOffset, c.AsyncBackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestTimeToLiveSecondsOffset, c.TimeToLiveSeconds)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestMaxIdleSecondsOffset, c.MaxIdleSeconds)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestReadBackupDataOffset, c.ReadBackupData)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestMergeBatchSizeOffset, c.MergeBatchSize)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestStatisticsEnabledOffset, c.StatisticsEnabled)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestMetadataPolicyOffset, int32(c.MetadataPolicy))
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestPerEntryStatsEnabledOffset, c.PerEntryStatsEnabled)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, c.Name)
	EncodeNullableForEvictionConfigHolder(clientMessage, c.EvictionConfig) //changed function signature
	EncodeString(clientMessage, string(c.CacheDeserializedValues))
	EncodeString(clientMessage, c.MergePolicy)
	EncodeString(clientMessage, string(c.InMemoryFormat))
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.ListenerConfigs)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.PartitionLostListenerConfigs)
	EncodeNullableForString(clientMessage, c.SplitBrainProtectionName)
	EncodeNullableForMapStoreConfigHolder(clientMessage, c.MapStoreConfig)   //changed function signature
	EncodeNullableForNearCacheConfigHolder(clientMessage, c.NearCacheConfig) //changed function signature
	EncodeNullableForWanReplicationRef(clientMessage, c.WanReplicationRef)   //changed function signature
	EncodeNullableListMultiFrameForIndexConfig(clientMessage, c.IndexConfigs)
	EncodeNullableListMultiFrameForAttributeConfig(clientMessage, c.AttributeConfigs)
	EncodeNullableListMultiFrameForQueryCacheConfigHolder(clientMessage, c.QueryCacheConfigs)
	EncodeNullableForString(clientMessage, c.PartitioningStrategyClassName)
	EncodeNullable(clientMessage, c.PartitioningStrategyImplementation, EncodeData)
	EncodeNullableForHotRestartConfig(clientMessage, c.HotRestartConfig)     //changed function signature
	EncodeNullableForEventJournalConfig(clientMessage, c.EventJournalConfig) //changed function signature
	EncodeNullableForMerkleTreeConfig(clientMessage, c.MerkleTreeConfig)     //changed function signature

	return clientMessage
}
