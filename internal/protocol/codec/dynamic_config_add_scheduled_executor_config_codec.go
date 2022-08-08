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
	DynamicConfigAddScheduledExecutorConfigCodecRequestMessageType  = int32(0x1B0A00)
	DynamicConfigAddScheduledExecutorConfigCodecResponseMessageType = int32(0x1B0A01)

	DynamicConfigAddScheduledExecutorConfigCodecRequestPoolSizeOffset          = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddScheduledExecutorConfigCodecRequestDurabilityOffset        = DynamicConfigAddScheduledExecutorConfigCodecRequestPoolSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddScheduledExecutorConfigCodecRequestCapacityOffset          = DynamicConfigAddScheduledExecutorConfigCodecRequestDurabilityOffset + proto.IntSizeInBytes
	DynamicConfigAddScheduledExecutorConfigCodecRequestMergeBatchSizeOffset    = DynamicConfigAddScheduledExecutorConfigCodecRequestCapacityOffset + proto.IntSizeInBytes
	DynamicConfigAddScheduledExecutorConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddScheduledExecutorConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddScheduledExecutorConfigCodecRequestInitialFrameSize        = DynamicConfigAddScheduledExecutorConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
)

// Adds a new scheduled executor configuration to a running cluster.
// If a scheduled executor configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddScheduledExecutorConfigRequest(es *types.ScheduledExecutorServiceConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddScheduledExecutorConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, DynamicConfigAddScheduledExecutorConfigCodecRequestPoolSizeOffset, es.PoolSize)
	EncodeInt(initialFrame.Content, DynamicConfigAddScheduledExecutorConfigCodecRequestDurabilityOffset, es.Durability)
	EncodeInt(initialFrame.Content, DynamicConfigAddScheduledExecutorConfigCodecRequestCapacityOffset, es.Capacity)
	EncodeInt(initialFrame.Content, DynamicConfigAddScheduledExecutorConfigCodecRequestMergeBatchSizeOffset, es.MergeBatchSize)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddScheduledExecutorConfigCodecRequestStatisticsEnabledOffset, es.StatisticsEnabled)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddScheduledExecutorConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, es.Name)
	EncodeNullableForString(clientMessage, es.SplitBrainProtectionName)
	EncodeString(clientMessage, es.MergePolicy)

	return clientMessage
}
