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
	proto "github.com/hazelcast/hazelcast-go-client"
)

const (
	MCDemoteDataMemberCodecRequestMessageType  = int32(0x202500)
	MCDemoteDataMemberCodecResponseMessageType = int32(0x202501)

	MCDemoteDataMemberCodecRequestInitialFrameSize = proto.PartitionIDOffset + proto.IntSizeInBytes
)

// Demotes the data member it's called on to a lite member.
// May throw an exception if the operation is not successful.

func EncodeMCDemoteDataMemberRequest() *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCDemoteDataMemberCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCDemoteDataMemberCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	return clientMessage
}
