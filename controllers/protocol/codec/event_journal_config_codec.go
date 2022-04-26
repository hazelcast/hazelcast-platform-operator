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
	EventJournalConfigCodecEnabledFieldOffset                = 0
	EventJournalConfigCodecCapacityFieldOffset               = EventJournalConfigCodecEnabledFieldOffset + proto.BooleanSizeInBytes
	EventJournalConfigCodecTimeToLiveSecondsFieldOffset      = EventJournalConfigCodecCapacityFieldOffset + proto.IntSizeInBytes
	EventJournalConfigCodecTimeToLiveSecondsInitialFrameSize = EventJournalConfigCodecTimeToLiveSecondsFieldOffset + proto.IntSizeInBytes
)

func EncodeEventJournalConfig(clientMessage *proto.ClientMessage, eventJournalConfig types.EventJournalConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, EventJournalConfigCodecTimeToLiveSecondsInitialFrameSize))
	EncodeBoolean(initialFrame.Content, EventJournalConfigCodecEnabledFieldOffset, eventJournalConfig.Enabled)
	EncodeInt(initialFrame.Content, EventJournalConfigCodecCapacityFieldOffset, int32(eventJournalConfig.Capacity))
	EncodeInt(initialFrame.Content, EventJournalConfigCodecTimeToLiveSecondsFieldOffset, int32(eventJournalConfig.TimeToLiveSeconds))
	clientMessage.AddFrame(initialFrame)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableForEventJournalConfig(clientMessage *proto.ClientMessage, eventJournalConfig types.EventJournalConfig) {
	if eventJournalConfig == (types.EventJournalConfig{}) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeEventJournalConfig(clientMessage, eventJournalConfig)
	}
}

func DecodeEventJournalConfig(frameIterator *proto.ForwardFrameIterator) types.EventJournalConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	enabled := DecodeBoolean(initialFrame.Content, EventJournalConfigCodecEnabledFieldOffset)
	capacity := DecodeInt(initialFrame.Content, EventJournalConfigCodecCapacityFieldOffset)
	timeToLiveSeconds := DecodeInt(initialFrame.Content, EventJournalConfigCodecTimeToLiveSecondsFieldOffset)
	FastForwardToEndFrame(frameIterator)

	return types.EventJournalConfig{
		Enabled:           enabled,
		Capacity:          capacity,
		TimeToLiveSeconds: timeToLiveSeconds,
	}
}
