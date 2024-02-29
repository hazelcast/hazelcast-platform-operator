/*
* Copyright (c) 2008-2024, Hazelcast, Inc. All Rights Reserved.
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
	iserialization "github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	CPMapPutCodecRequestMessageType  = int32(0x230200)
	CPMapPutCodecResponseMessageType = int32(0x230201)

	CPMapPutCodecRequestInitialFrameSize = proto.PartitionIDOffset + proto.IntSizeInBytes
)

// Puts the key-value into the specified map.

func EncodeCPMapPutRequest(groupId types.RaftGroupId, name string, key iserialization.Data, value iserialization.Data) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, CPMapPutCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(CPMapPutCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeRaftGroupId(clientMessage, groupId)
	EncodeString(clientMessage, name)
	EncodeData(clientMessage, key)
	EncodeData(clientMessage, value)

	return clientMessage
}

func DecodeCPMapPutResponse(clientMessage *proto.ClientMessage) iserialization.Data {
	frameIterator := clientMessage.FrameIterator()
	// empty initial frame
	frameIterator.Next()

	return DecodeNullableForData(frameIterator)
}
