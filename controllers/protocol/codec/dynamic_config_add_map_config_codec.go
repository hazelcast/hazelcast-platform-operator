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
	iserialization "github.com/hazelcast/hazelcast-go-client"
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

func EncodeDynamicConfigAddMapConfigRequest(
	name string,
	backupCount int32,
	asyncBackupCount int32,
	timeToLiveSeconds int32,
	maxIdleSeconds int32,
	evictionConfig types.EvictionConfigHolder,
	readBackupData bool,
	cacheDeserializedValues string,
	mergePolicy string,
	mergeBatchSize int32,
	inMemoryFormat string,
	listenerConfigs []types.ListenerConfigHolder,
	partitionLostListenerConfigs []types.ListenerConfigHolder,
	statisticsEnabled bool,
	splitBrainProtectionName string,
	mapStoreConfig types.MapStoreConfigHolder,
	nearCacheConfig types.NearCacheConfigHolder,
	wanReplicationRef types.WanReplicationRef,
	indexConfigs []types.IndexConfig,
	attributeConfigs []types.AttributeConfig,
	queryCacheConfigs []types.QueryCacheConfigHolder,
	partitioningStrategyClassName string,
	partitioningStrategyImplementation iserialization.Data,
	hotRestartConfig types.HotRestartConfig,
	eventJournalConfig types.EventJournalConfig,
	merkleTreeConfig types.MerkleTreeConfig,
	metadataPolicy int32,
	perEntryStatsEnabled bool) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestBackupCountOffset, backupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestAsyncBackupCountOffset, asyncBackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestTimeToLiveSecondsOffset, timeToLiveSeconds)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestMaxIdleSecondsOffset, maxIdleSeconds)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestReadBackupDataOffset, readBackupData)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestMergeBatchSizeOffset, mergeBatchSize)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestStatisticsEnabledOffset, statisticsEnabled)
	EncodeInt(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestMetadataPolicyOffset, metadataPolicy)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMapConfigCodecRequestPerEntryStatsEnabledOffset, perEntryStatsEnabled)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, name)
	EncodeNullableForEvictionConfigHolder(clientMessage, evictionConfig) //changed function signature
	EncodeString(clientMessage, cacheDeserializedValues)
	EncodeString(clientMessage, mergePolicy)
	EncodeString(clientMessage, inMemoryFormat)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, listenerConfigs)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, partitionLostListenerConfigs)
	EncodeNullableForString(clientMessage, splitBrainProtectionName)
	EncodeNullableForMapStoreConfigHolder(clientMessage, mapStoreConfig)   //changed function signature
	EncodeNullableForNearCacheConfigHolder(clientMessage, nearCacheConfig) //changed function signature
	EncodeNullableForWanReplicationRef(clientMessage, wanReplicationRef)   //changed function signature
	EncodeNullableListMultiFrameForIndexConfig(clientMessage, indexConfigs)
	EncodeNullableListMultiFrameForAttributeConfig(clientMessage, attributeConfigs)
	EncodeNullableListMultiFrameForQueryCacheConfigHolder(clientMessage, queryCacheConfigs)
	EncodeNullableForString(clientMessage, partitioningStrategyClassName)
	EncodeNullable(clientMessage, partitioningStrategyImplementation, EncodeData)
	EncodeNullableForHotRestartConfig(clientMessage, hotRestartConfig)     //changed function signature
	EncodeNullableForEventJournalConfig(clientMessage, eventJournalConfig) //changed function signature
	EncodeNullableForMerkleTreeConfig(clientMessage, merkleTreeConfig)     //changed function signature

	return clientMessage
}
