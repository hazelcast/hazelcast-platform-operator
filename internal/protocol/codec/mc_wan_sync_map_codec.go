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
	clientTypes "github.com/hazelcast/hazelcast-go-client/types"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	MCWanSyncMapCodecRequestMessageType  = int32(0x201600)
	MCWanSyncMapCodecResponseMessageType = int32(0x201601)

	MCWanSyncMapCodecRequestWanSyncTypeOffset = proto.PartitionIDOffset + proto.IntSizeInBytes
	MCWanSyncMapCodecRequestInitialFrameSize  = MCWanSyncMapCodecRequestWanSyncTypeOffset + proto.IntSizeInBytes

	MCWanSyncMapResponseUuidOffset = proto.ResponseBackupAcksOffset + proto.ByteSizeInBytes
)

// Initiate WAN sync for a specific map or all maps

func EncodeMCWanSyncMapRequest(wanSync types.WanSyncRef) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCWanSyncMapCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, MCWanSyncMapCodecRequestWanSyncTypeOffset, int32(wanSync.Type))
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCWanSyncMapCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, wanSync.WanReplicationName)
	EncodeString(clientMessage, wanSync.WanPublisherId)
	EncodeNullableForString(clientMessage, wanSync.MapName)

	return clientMessage
}

func DecodeMCWanSyncMapResponse(clientMessage *proto.ClientMessage) clientTypes.UUID {
	frameIterator := clientMessage.FrameIterator()
	initialFrame := frameIterator.Next()

	return DecodeUUID(initialFrame.Content, MCWanSyncMapResponseUuidOffset)
}
