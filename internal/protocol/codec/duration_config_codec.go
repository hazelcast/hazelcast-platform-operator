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
	DurationConfigCodecDurationAmountFieldOffset = 0
	DurationConfigCodecTimeUnitFieldOffset       = DurationConfigCodecDurationAmountFieldOffset + proto.LongSizeInBytes
	DurationConfigCodecTimeUnitInitialFrameSize  = DurationConfigCodecTimeUnitFieldOffset + proto.IntSizeInBytes
)

func EncodeDurationConfig(clientMessage *proto.ClientMessage, durationConfig types.DurationConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, DurationConfigCodecTimeUnitInitialFrameSize))
	EncodeLong(initialFrame.Content, DurationConfigCodecDurationAmountFieldOffset, int64(durationConfig.DurationAmount))
	EncodeInt(initialFrame.Content, DurationConfigCodecTimeUnitFieldOffset, int32(durationConfig.TimeUnit))
	clientMessage.AddFrame(initialFrame)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func DecodeDurationConfig(frameIterator *proto.ForwardFrameIterator) types.DurationConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	durationAmount := DecodeLong(initialFrame.Content, DurationConfigCodecDurationAmountFieldOffset)
	timeUnit := DecodeInt(initialFrame.Content, DurationConfigCodecTimeUnitFieldOffset)
	FastForwardToEndFrame(frameIterator)

	return types.DurationConfig{
		DurationAmount: durationAmount,
		TimeUnit:       timeUnit,
	}
}
