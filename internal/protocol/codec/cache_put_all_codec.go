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
	CachePutAllCodecRequestMessageType  = int32(0x131B00)
	CachePutAllCodecResponseMessageType = int32(0x131B01)

	CachePutAllCodecRequestCompletionIdOffset = proto.PartitionIDOffset + proto.IntSizeInBytes
	CachePutAllCodecRequestInitialFrameSize   = CachePutAllCodecRequestCompletionIdOffset + proto.IntSizeInBytes
)

// Copies all the mappings from the specified map to this cache with the given expiry policy.

func EncodeCachePutAllRequest(name string, entries []proto.Pair, expiryPolicy iserialization.Data, completionId int32) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, CachePutAllCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, CachePutAllCodecRequestCompletionIdOffset, completionId)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(CachePutAllCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, name)
	EncodeEntryListForDataAndData(clientMessage, entries)
	EncodeNullable(clientMessage, expiryPolicy, EncodeData)

	return clientMessage
}
