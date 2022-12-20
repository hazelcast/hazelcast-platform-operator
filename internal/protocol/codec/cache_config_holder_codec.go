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
	CacheConfigHolderCodecBackupCountFieldOffset                            = 0
	CacheConfigHolderCodecAsyncBackupCountFieldOffset                       = CacheConfigHolderCodecBackupCountFieldOffset + proto.IntSizeInBytes
	CacheConfigHolderCodecReadThroughFieldOffset                            = CacheConfigHolderCodecAsyncBackupCountFieldOffset + proto.IntSizeInBytes
	CacheConfigHolderCodecWriteThroughFieldOffset                           = CacheConfigHolderCodecReadThroughFieldOffset + proto.BooleanSizeInBytes
	CacheConfigHolderCodecStoreByValueFieldOffset                           = CacheConfigHolderCodecWriteThroughFieldOffset + proto.BooleanSizeInBytes
	CacheConfigHolderCodecManagementEnabledFieldOffset                      = CacheConfigHolderCodecStoreByValueFieldOffset + proto.BooleanSizeInBytes
	CacheConfigHolderCodecStatisticsEnabledFieldOffset                      = CacheConfigHolderCodecManagementEnabledFieldOffset + proto.BooleanSizeInBytes
	CacheConfigHolderCodecDisablePerEntryInvalidationEventsFieldOffset      = CacheConfigHolderCodecStatisticsEnabledFieldOffset + proto.BooleanSizeInBytes
	CacheConfigHolderCodecDisablePerEntryInvalidationEventsInitialFrameSize = CacheConfigHolderCodecDisablePerEntryInvalidationEventsFieldOffset + proto.BooleanSizeInBytes
)

func EncodeCacheConfigHolder(clientMessage *proto.ClientMessage, cacheConfigHolder types.CacheConfigHolder) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, CacheConfigHolderCodecDisablePerEntryInvalidationEventsInitialFrameSize))
	EncodeInt(initialFrame.Content, CacheConfigHolderCodecBackupCountFieldOffset, int32(cacheConfigHolder.BackupCount))
	EncodeInt(initialFrame.Content, CacheConfigHolderCodecAsyncBackupCountFieldOffset, int32(cacheConfigHolder.AsyncBackupCount))
	EncodeBoolean(initialFrame.Content, CacheConfigHolderCodecReadThroughFieldOffset, cacheConfigHolder.ReadThrough)
	EncodeBoolean(initialFrame.Content, CacheConfigHolderCodecWriteThroughFieldOffset, cacheConfigHolder.WriteThrough)
	EncodeBoolean(initialFrame.Content, CacheConfigHolderCodecStoreByValueFieldOffset, cacheConfigHolder.StoreByValue)
	EncodeBoolean(initialFrame.Content, CacheConfigHolderCodecManagementEnabledFieldOffset, cacheConfigHolder.ManagementEnabled)
	EncodeBoolean(initialFrame.Content, CacheConfigHolderCodecStatisticsEnabledFieldOffset, cacheConfigHolder.StatisticsEnabled)
	EncodeBoolean(initialFrame.Content, CacheConfigHolderCodecDisablePerEntryInvalidationEventsFieldOffset, cacheConfigHolder.DisablePerEntryInvalidationEvents)
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, cacheConfigHolder.Name)
	EncodeNullableForString(clientMessage, cacheConfigHolder.ManagerPrefix)
	EncodeNullableForString(clientMessage, cacheConfigHolder.UriString)
	EncodeString(clientMessage, cacheConfigHolder.InMemoryFormat)
	EncodeEvictionConfigHolder(clientMessage, cacheConfigHolder.EvictionConfigHolder)
	EncodeNullableForWanReplicationRef(clientMessage, cacheConfigHolder.WanReplicationRef)
	EncodeString(clientMessage, cacheConfigHolder.KeyClassName)
	EncodeString(clientMessage, cacheConfigHolder.ValueClassName)
	EncodeNullableForData(clientMessage, cacheConfigHolder.CacheLoaderFactory)
	EncodeNullableForData(clientMessage, cacheConfigHolder.CacheWriterFactory)
	EncodeData(clientMessage, cacheConfigHolder.ExpiryPolicyFactory)
	EncodeNullableForHotRestartConfig(clientMessage, cacheConfigHolder.HotRestartConfig)
	EncodeNullableForEventJournalConfig(clientMessage, cacheConfigHolder.EventJournalConfig)
	EncodeNullableForString(clientMessage, cacheConfigHolder.SplitBrainProtectionName)
	EncodeListMultiFrameForData(clientMessage, cacheConfigHolder.ListenerConfigurations)
	EncodeMergePolicyConfig(clientMessage, cacheConfigHolder.MergePolicyConfig)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, cacheConfigHolder.CachePartitionLostListenerConfigs)
	EncodeNullableForMerkleTreeConfig(clientMessage, cacheConfigHolder.MerkleTreeConfig)
	EncodeDataPersistenceConfig(clientMessage, cacheConfigHolder.DataPersistenceConfig)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func DecodeNullableForCacheConfigHolder(frameIterator *proto.ForwardFrameIterator) types.CacheConfigHolder {
	if NextFrameIsNullFrame(frameIterator) {
		return types.CacheConfigHolder{}
	}
	return DecodeCacheConfigHolder(frameIterator)
}

func DecodeCacheConfigHolder(frameIterator *proto.ForwardFrameIterator) types.CacheConfigHolder {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	backupCount := DecodeInt(initialFrame.Content, CacheConfigHolderCodecBackupCountFieldOffset)
	asyncBackupCount := DecodeInt(initialFrame.Content, CacheConfigHolderCodecAsyncBackupCountFieldOffset)
	readThrough := DecodeBoolean(initialFrame.Content, CacheConfigHolderCodecReadThroughFieldOffset)
	writeThrough := DecodeBoolean(initialFrame.Content, CacheConfigHolderCodecWriteThroughFieldOffset)
	storeByValue := DecodeBoolean(initialFrame.Content, CacheConfigHolderCodecStoreByValueFieldOffset)
	managementEnabled := DecodeBoolean(initialFrame.Content, CacheConfigHolderCodecManagementEnabledFieldOffset)
	statisticsEnabled := DecodeBoolean(initialFrame.Content, CacheConfigHolderCodecStatisticsEnabledFieldOffset)
	disablePerEntryInvalidationEvents := DecodeBoolean(initialFrame.Content, CacheConfigHolderCodecDisablePerEntryInvalidationEventsFieldOffset)

	name := DecodeString(frameIterator)
	managerPrefix := DecodeNullableForString(frameIterator)
	uriString := DecodeNullableForString(frameIterator)
	inMemoryFormat := DecodeString(frameIterator)
	evictionConfigHolder := DecodeEvictionConfigHolder(frameIterator)
	wanReplicationRef := DecodeWanReplicationRef(frameIterator)
	keyClassName := DecodeString(frameIterator)
	valueClassName := DecodeString(frameIterator)
	cacheLoaderFactory := DecodeNullableForData(frameIterator)
	cacheWriterFactory := DecodeNullableForData(frameIterator)
	expiryPolicyFactory := DecodeData(frameIterator)
	hotRestartConfig := DecodeHotRestartConfig(frameIterator)
	eventJournalConfig := DecodeEventJournalConfig(frameIterator)
	splitBrainProtectionName := DecodeNullableForString(frameIterator)
	listenerConfigurations := DecodeListMultiFrameForDataContainsNullable(frameIterator)
	mergePolicyConfig := DecodeMergePolicyConfig(frameIterator)
	cachePartitionLostListenerConfigs := DecodeNullableListMultiFrameForListenerConfigHolder(frameIterator)
	var merkleTreeConfig types.MerkleTreeConfig
	if !frameIterator.PeekNext().IsEndFrame() {
		merkleTreeConfig = DecodeMerkleTreeConfig(frameIterator)
	}
	var dataPersistenceConfig types.DataPersistenceConfig
	if !frameIterator.PeekNext().IsEndFrame() {
		dataPersistenceConfig = DecodeDataPersistenceConfig(frameIterator)
	}
	FastForwardToEndFrame(frameIterator)

	return types.CacheConfigHolder{
		Name:                              name,
		ManagerPrefix:                     managerPrefix,
		UriString:                         uriString,
		BackupCount:                       backupCount,
		AsyncBackupCount:                  asyncBackupCount,
		InMemoryFormat:                    inMemoryFormat,
		EvictionConfigHolder:              evictionConfigHolder,
		WanReplicationRef:                 wanReplicationRef,
		KeyClassName:                      keyClassName,
		ValueClassName:                    valueClassName,
		CacheLoaderFactory:                cacheLoaderFactory,
		CacheWriterFactory:                cacheWriterFactory,
		ExpiryPolicyFactory:               expiryPolicyFactory,
		ReadThrough:                       readThrough,
		WriteThrough:                      writeThrough,
		StoreByValue:                      storeByValue,
		ManagementEnabled:                 managementEnabled,
		StatisticsEnabled:                 statisticsEnabled,
		HotRestartConfig:                  hotRestartConfig,
		EventJournalConfig:                eventJournalConfig,
		SplitBrainProtectionName:          splitBrainProtectionName,
		ListenerConfigurations:            listenerConfigurations,
		MergePolicyConfig:                 mergePolicyConfig,
		DisablePerEntryInvalidationEvents: disablePerEntryInvalidationEvents,
		CachePartitionLostListenerConfigs: cachePartitionLostListenerConfigs,
		MerkleTreeConfig:                  merkleTreeConfig,
		DataPersistenceConfig:             dataPersistenceConfig,
	}
}
