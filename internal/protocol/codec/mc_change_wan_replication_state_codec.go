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
)

const (
	MCChangeWanReplicationStateCodecRequestMessageType  = int32(0x201300)
	MCChangeWanReplicationStateCodecResponseMessageType = int32(0x201301)

	MCChangeWanReplicationStateCodecRequestNewStateOffset   = proto.PartitionIDOffset + proto.IntSizeInBytes
	MCChangeWanReplicationStateCodecRequestInitialFrameSize = MCChangeWanReplicationStateCodecRequestNewStateOffset + proto.ByteSizeInBytes
)

// Stop, pause or resume WAN replication for the given WAN replication and publisher

func EncodeMCChangeWanReplicationStateRequest(wanReplicationName string, wanPublisherId string, newState byte) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCChangeWanReplicationStateCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeByte(initialFrame.Content, MCChangeWanReplicationStateCodecRequestNewStateOffset, newState)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCChangeWanReplicationStateCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, wanReplicationName)
	EncodeString(clientMessage, wanPublisherId)

	return clientMessage
}
