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
	iserialization "github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
)

const (
	CachePutCodecRequestMessageType  = int32(0x131300)
	CachePutCodecResponseMessageType = int32(0x131301)

	CachePutCodecRequestGetOffset          = proto.PartitionIDOffset + proto.IntSizeInBytes
	CachePutCodecRequestCompletionIdOffset = CachePutCodecRequestGetOffset + proto.BooleanSizeInBytes
	CachePutCodecRequestInitialFrameSize   = CachePutCodecRequestCompletionIdOffset + proto.IntSizeInBytes
)

// Puts the entry with the given key, value and the expiry policy to the cache.

func EncodeCachePutRequest(name string, key interface{}, value interface{}, expiryPolicy iserialization.Data, get bool, completionId int32) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, CachePutCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, CachePutCodecRequestGetOffset, get)
	EncodeInt(initialFrame.Content, CachePutCodecRequestCompletionIdOffset, completionId)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(CachePutCodecRequestMessageType)
	clientMessage.SetPartitionId(1)

	EncodeString(clientMessage, name)
	EncodeData(clientMessage, key)
	EncodeData(clientMessage, value)
	EncodeNullable(clientMessage, expiryPolicy, EncodeData)

	return clientMessage
}

func DecodeCachePutResponse(clientMessage *proto.ClientMessage) iserialization.Data {
	frameIterator := clientMessage.FrameIterator()
	// empty initial frame
	frameIterator.Next()

	return DecodeNullableForData(frameIterator)
}
