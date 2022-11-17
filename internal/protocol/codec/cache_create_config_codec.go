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
	CacheCreateConfigCodecRequestMessageType  = int32(0x130600)
	CacheCreateConfigCodecResponseMessageType = int32(0x130601)

	CacheCreateConfigCodecRequestCreateAlsoOnOthersOffset = proto.PartitionIDOffset + proto.IntSizeInBytes
	CacheCreateConfigCodecRequestInitialFrameSize         = CacheCreateConfigCodecRequestCreateAlsoOnOthersOffset + proto.BooleanSizeInBytes
)

// Creates the given cache configuration on Hazelcast members.

func EncodeCacheCreateConfigRequest(cacheConfig types.CacheConfigHolder, createAlsoOnOthers bool) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(true)

	initialFrame := proto.NewFrameWith(make([]byte, CacheCreateConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, CacheCreateConfigCodecRequestCreateAlsoOnOthersOffset, createAlsoOnOthers)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(CacheCreateConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeCacheConfigHolder(clientMessage, cacheConfig)

	return clientMessage
}

func DecodeCacheCreateConfigResponse(clientMessage *proto.ClientMessage) types.CacheConfigHolder {
	frameIterator := clientMessage.FrameIterator()
	// empty initial frame
	frameIterator.Next()
	return DecodeCacheConfigHolder(frameIterator)
}
