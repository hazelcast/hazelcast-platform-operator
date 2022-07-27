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
	DynamicConfigAddDurableExecutorConfigCodecRequestMessageType  = int32(0x1B0900)
	DynamicConfigAddDurableExecutorConfigCodecResponseMessageType = int32(0x1B0901)

	DynamicConfigAddDurableExecutorConfigCodecRequestPoolSizeOffset          = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddDurableExecutorConfigCodecRequestDurabilityOffset        = DynamicConfigAddDurableExecutorConfigCodecRequestPoolSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddDurableExecutorConfigCodecRequestCapacityOffset          = DynamicConfigAddDurableExecutorConfigCodecRequestDurabilityOffset + proto.IntSizeInBytes
	DynamicConfigAddDurableExecutorConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddDurableExecutorConfigCodecRequestCapacityOffset + proto.IntSizeInBytes
	DynamicConfigAddDurableExecutorConfigCodecRequestInitialFrameSize        = DynamicConfigAddDurableExecutorConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
)

// Adds a new durable executor configuration to a running cluster.
// If a durable executor configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddDurableExecutorConfigRequest(es *types.AddDurableExecutorInput) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddDurableExecutorConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, DynamicConfigAddDurableExecutorConfigCodecRequestPoolSizeOffset, es.PoolSize)
	EncodeInt(initialFrame.Content, DynamicConfigAddDurableExecutorConfigCodecRequestDurabilityOffset, es.Durability)
	EncodeInt(initialFrame.Content, DynamicConfigAddDurableExecutorConfigCodecRequestCapacityOffset, es.Capacity)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddDurableExecutorConfigCodecRequestStatisticsEnabledOffset, es.StatisticsEnabled)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddDurableExecutorConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, es.Name)
	EncodeNullableForString(clientMessage, es.SplitBrainProtectionName)

	return clientMessage
}
