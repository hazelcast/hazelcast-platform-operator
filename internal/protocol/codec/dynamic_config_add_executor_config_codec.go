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
	DynamicConfigAddExecutorConfigCodecRequestMessageType  = int32(0x1B0800)
	DynamicConfigAddExecutorConfigCodecResponseMessageType = int32(0x1B0801)

	DynamicConfigAddExecutorConfigCodecRequestPoolSizeOffset          = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddExecutorConfigCodecRequestQueueCapacityOffset     = DynamicConfigAddExecutorConfigCodecRequestPoolSizeOffset + proto.IntSizeInBytes
	DynamicConfigAddExecutorConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddExecutorConfigCodecRequestQueueCapacityOffset + proto.IntSizeInBytes
	DynamicConfigAddExecutorConfigCodecRequestInitialFrameSize        = DynamicConfigAddExecutorConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
)

// Adds a new executor configuration to a running cluster.
// If an executor configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddExecutorConfigRequest(es *types.ExecutorServiceConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddExecutorConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, DynamicConfigAddExecutorConfigCodecRequestPoolSizeOffset, es.PoolSize)
	EncodeInt(initialFrame.Content, DynamicConfigAddExecutorConfigCodecRequestQueueCapacityOffset, es.QueueCapacity)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddExecutorConfigCodecRequestStatisticsEnabledOffset, es.StatisticsEnabled)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddExecutorConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, es.Name)
	EncodeNullableForString(clientMessage, es.SplitBrainProtectionName)

	return clientMessage
}
