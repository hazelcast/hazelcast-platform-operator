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
	MCUpdateMapConfigCodecRequestMessageType  = int32(0x200400)
	MCUpdateMapConfigCodecResponseMessageType = int32(0x200401)

	MCUpdateMapConfigCodecRequestTimeToLiveSecondsOffset = proto.PartitionIDOffset + proto.IntSizeInBytes
	MCUpdateMapConfigCodecRequestMaxIdleSecondsOffset    = MCUpdateMapConfigCodecRequestTimeToLiveSecondsOffset + proto.IntSizeInBytes
	MCUpdateMapConfigCodecRequestEvictionPolicyOffset    = MCUpdateMapConfigCodecRequestMaxIdleSecondsOffset + proto.IntSizeInBytes
	MCUpdateMapConfigCodecRequestReadBackupDataOffset    = MCUpdateMapConfigCodecRequestEvictionPolicyOffset + proto.IntSizeInBytes
	MCUpdateMapConfigCodecRequestMaxSizeOffset           = MCUpdateMapConfigCodecRequestReadBackupDataOffset + proto.BooleanSizeInBytes
	MCUpdateMapConfigCodecRequestMaxSizePolicyOffset     = MCUpdateMapConfigCodecRequestMaxSizeOffset + proto.IntSizeInBytes
	MCUpdateMapConfigCodecRequestInitialFrameSize        = MCUpdateMapConfigCodecRequestMaxSizePolicyOffset + proto.IntSizeInBytes
)

// Updates the config of a map on the member it's called on.

func EncodeMCUpdateMapConfigRequest(mapName string, timeToLiveSeconds int32, maxIdleSeconds int32, evictionPolicy int32, readBackupData bool, maxSize int32, maxSizePolicy int32) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCUpdateMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, MCUpdateMapConfigCodecRequestTimeToLiveSecondsOffset, timeToLiveSeconds)
	EncodeInt(initialFrame.Content, MCUpdateMapConfigCodecRequestMaxIdleSecondsOffset, maxIdleSeconds)
	EncodeInt(initialFrame.Content, MCUpdateMapConfigCodecRequestEvictionPolicyOffset, evictionPolicy)
	EncodeBoolean(initialFrame.Content, MCUpdateMapConfigCodecRequestReadBackupDataOffset, readBackupData)
	EncodeInt(initialFrame.Content, MCUpdateMapConfigCodecRequestMaxSizeOffset, maxSize)
	EncodeInt(initialFrame.Content, MCUpdateMapConfigCodecRequestMaxSizePolicyOffset, maxSizePolicy)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCUpdateMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, mapName)

	return clientMessage
}
