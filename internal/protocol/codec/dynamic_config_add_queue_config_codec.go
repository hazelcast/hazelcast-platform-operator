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
	DynamicConfigAddQueueConfigCodecRequestMessageType  = int32(0x1B0B00)
	DynamicConfigAddQueueConfigCodecResponseMessageType = int32(0x1B0B01)

	DynamicConfigAddQueueConfigCodecRequestBackupCountOffset       = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddQueueConfigCodecRequestAsyncBackupCountOffset  = DynamicConfigAddQueueConfigCodecRequestBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddQueueConfigCodecRequestMaxSizeOffset           = DynamicConfigAddQueueConfigCodecRequestAsyncBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddQueueConfigCodecRequestEmptyQueueTtlOffset     = DynamicConfigAddQueueConfigCodecRequestMaxSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddQueueConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddQueueConfigCodecRequestEmptyQueueTtlOffset + proto.IntSizeInBytes
	DynamicConfigAddQueueConfigCodecRequestMergeBatchSizeOffset    = DynamicConfigAddQueueConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddQueueConfigCodecRequestInitialFrameSize        = DynamicConfigAddQueueConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
)

// Adds a new queue configuration to a running cluster.
// If a queue configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddQueueConfigRequest(input *types.QueueConfigInput) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddQueueConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, DynamicConfigAddQueueConfigCodecRequestBackupCountOffset, input.BackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddQueueConfigCodecRequestAsyncBackupCountOffset, input.AsyncBackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddQueueConfigCodecRequestMaxSizeOffset, input.MaxSize)
	EncodeInt(initialFrame.Content, DynamicConfigAddQueueConfigCodecRequestEmptyQueueTtlOffset, input.EmptyQueueTtl)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddQueueConfigCodecRequestStatisticsEnabledOffset, input.StatisticsEnabled)
	EncodeInt(initialFrame.Content, DynamicConfigAddQueueConfigCodecRequestMergeBatchSizeOffset, input.MergeBatchSize)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddQueueConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, input.Name)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, input.ListenerConfigs)
	EncodeNullableForString(clientMessage, input.SplitBrainProtectionName)
	EncodeNullableForQueueStoreConfigHolder(clientMessage, input.QueueStoreConfig)
	EncodeString(clientMessage, input.MergePolicy)
	EncodeNullableForString(clientMessage, input.PriorityComparatorClassName)

	return clientMessage
}

//manual
func EncodeNullableForQueueStoreConfigHolder(clientMessage *proto.ClientMessage, _ types.QueueStoreConfigHolder) {
	// Not implemented
	clientMessage.AddFrame(proto.NullFrame.Copy())
}
