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

	types "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	DynamicConfigAddTopicConfigCodecRequestMessageType  = int32(0x1B0700)
	DynamicConfigAddTopicConfigCodecResponseMessageType = int32(0x1B0701)

	DynamicConfigAddTopicConfigCodecRequestGlobalOrderingEnabledOffset = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddTopicConfigCodecRequestStatisticsEnabledOffset     = DynamicConfigAddTopicConfigCodecRequestGlobalOrderingEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddTopicConfigCodecRequestMultiThreadingEnabledOffset = DynamicConfigAddTopicConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddTopicConfigCodecRequestInitialFrameSize            = DynamicConfigAddTopicConfigCodecRequestMultiThreadingEnabledOffset + proto.BooleanSizeInBytes
)

// Adds a new topic configuration to a running cluster.
// If a topic configuration with the given {@code name} already exists, then
// the new configuration is ignored and the existing one is preserved.

func EncodeDynamicConfigAddTopicConfigRequest(t *types.TopicConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddTopicConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddTopicConfigCodecRequestGlobalOrderingEnabledOffset, t.GlobalOrderingEnabled)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddTopicConfigCodecRequestStatisticsEnabledOffset, t.StatisticsEnabled)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddTopicConfigCodecRequestMultiThreadingEnabledOffset, t.MultiThreadingEnabled)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddTopicConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, t.Name)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, t.ListenerConfigs)

	return clientMessage
}
