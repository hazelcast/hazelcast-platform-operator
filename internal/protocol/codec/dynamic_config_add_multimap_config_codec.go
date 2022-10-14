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
	DynamicConfigAddMultiMapConfigCodecRequestMessageType  = int32(0x1B0100)
	DynamicConfigAddMultiMapConfigCodecResponseMessageType = int32(0x1B0101)

	DynamicConfigAddMultiMapConfigCodecRequestBinaryOffset            = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestBackupCountOffset       = DynamicConfigAddMultiMapConfigCodecRequestBinaryOffset + proto.BooleanSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestAsyncBackupCountOffset  = DynamicConfigAddMultiMapConfigCodecRequestBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddMultiMapConfigCodecRequestAsyncBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestMergeBatchSizeOffset    = DynamicConfigAddMultiMapConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestInitialFrameSize        = DynamicConfigAddMultiMapConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
)

// Adds a new multimap config to a running cluster.
// If a multimap configuration with the given {@code name} already exists, then
// the new multimap config is ignored and the existing one is preserved.

func EncodeDynamicConfigAddMultiMapConfigRequest(c *types.MultiMapConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddMultiMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestBinaryOffset, c.Binary)
	EncodeInt(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestBackupCountOffset, c.BackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestAsyncBackupCountOffset, c.AsyncBackupCount)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestStatisticsEnabledOffset, c.StatisticsEnabled)
	EncodeInt(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestMergeBatchSizeOffset, c.MergeBatchSize)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddMultiMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, c.Name)
	EncodeString(clientMessage, c.CollectionType)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.ListenerConfigs)
	EncodeNullableForString(clientMessage, c.SplitBrainProtectionName)
	EncodeString(clientMessage, c.MergePolicy)

	return clientMessage
}
