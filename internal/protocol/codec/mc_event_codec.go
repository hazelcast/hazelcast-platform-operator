/*
* Copyright (c) 2008-2023, Hazelcast, Inc. All Rights Reserved.
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
	MCEventCodecTimestampFieldOffset = 0
	MCEventCodecTypeFieldOffset      = MCEventCodecTimestampFieldOffset + proto.LongSizeInBytes
	MCEventCodecTypeInitialFrameSize = MCEventCodecTypeFieldOffset + proto.IntSizeInBytes
)

func EncodeMCEvent(clientMessage *proto.ClientMessage, mCEvent types.MCEvent) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, MCEventCodecTypeInitialFrameSize))
	EncodeLong(initialFrame.Content, MCEventCodecTimestampFieldOffset, mCEvent.Timestamp)
	EncodeInt(initialFrame.Content, MCEventCodecTypeFieldOffset, int32(mCEvent.Type))
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, mCEvent.DataJson)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func DecodeMCEvent(frameIterator *proto.ForwardFrameIterator) types.MCEvent {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	timestamp := DecodeLong(initialFrame.Content, MCEventCodecTimestampFieldOffset)
	_type := DecodeInt(initialFrame.Content, MCEventCodecTypeFieldOffset)

	dataJson := DecodeString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.NewMCEvent(timestamp, types.MCEventType(_type), dataJson)
}
