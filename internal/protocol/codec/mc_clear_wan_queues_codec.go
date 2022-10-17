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
	MCClearWanQueuesCodecRequestMessageType  = int32(0x201400)
	MCClearWanQueuesCodecResponseMessageType = int32(0x201401)

	MCClearWanQueuesCodecRequestInitialFrameSize = proto.PartitionIDOffset + proto.IntSizeInBytes
)

// Clear WAN replication queues for the given wan replication and publisher

func EncodeMCClearWanQueuesRequest(wanReplicationName string, wanPublisherId string) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCClearWanQueuesCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCClearWanQueuesCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, wanReplicationName)
	EncodeString(clientMessage, wanPublisherId)

	return clientMessage
}
