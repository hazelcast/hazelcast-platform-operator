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
	MCPollMCEventsCodecRequestMessageType  = int32(0x201800)
	MCPollMCEventsCodecResponseMessageType = int32(0x201801)

	MCPollMCEventsCodecRequestInitialFrameSize = proto.PartitionIDOffset + proto.IntSizeInBytes
)

// Polls events available on member. Once read, events are removed from
// member's internal queue.

func EncodeMCPollMCEventsRequest() *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCPollMCEventsCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCPollMCEventsCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	return clientMessage
}

func DecodeMCPollMCEventsResponse(clientMessage *proto.ClientMessage) []types.MCEvent {
	frameIterator := clientMessage.FrameIterator()
	// empty initial frame
	frameIterator.Next()

	return DecodeListMultiFrameForMCEvent(frameIterator)
}
