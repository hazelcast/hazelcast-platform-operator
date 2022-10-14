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
	DynamicConfigAddReplicatedMapConfigCodecRequestMessageType  = int32(0x1B0600)
	DynamicConfigAddReplicatedMapConfigCodecResponseMessageType = int32(0x1B0601)

	DynamicConfigAddReplicatedMapConfigCodecRequestAsyncFillupOffset       = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddReplicatedMapConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddReplicatedMapConfigCodecRequestAsyncFillupOffset + proto.BooleanSizeInBytes
	DynamicConfigAddReplicatedMapConfigCodecRequestMergeBatchSizeOffset    = DynamicConfigAddReplicatedMapConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddReplicatedMapConfigCodecRequestInitialFrameSize        = DynamicConfigAddReplicatedMapConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
)

// Adds a new replicated map configuration to a running cluster.
// If a replicated map configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddReplicatedMapConfigRequest(c *types.ReplicatedMapConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddReplicatedMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddReplicatedMapConfigCodecRequestAsyncFillupOffset, c.AsyncFillup)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddReplicatedMapConfigCodecRequestStatisticsEnabledOffset, c.StatisticsEnabled)
	EncodeInt(initialFrame.Content, DynamicConfigAddReplicatedMapConfigCodecRequestMergeBatchSizeOffset, c.MergeBatchSize)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddReplicatedMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, c.Name)
	EncodeString(clientMessage, c.InMemoryFormat)
	EncodeString(clientMessage, c.MergePolicy)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.ListenerConfigs)
	EncodeNullableForString(clientMessage, c.SplitBrainProtectionName)

	return clientMessage
}
