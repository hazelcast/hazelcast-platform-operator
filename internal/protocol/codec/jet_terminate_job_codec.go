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

	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	JetTerminateJobCodecRequestMessageType  = int32(0xFE0200)
	JetTerminateJobCodecResponseMessageType = int32(0xFE0201)

	JetTerminateJobCodecRequestJobIdOffset               = proto.PartitionIDOffset + proto.IntSizeInBytes
	JetTerminateJobCodecRequestTerminateModeOffset       = JetTerminateJobCodecRequestJobIdOffset + proto.LongSizeInBytes
	JetTerminateJobCodecRequestLightJobCoordinatorOffset = JetTerminateJobCodecRequestTerminateModeOffset + proto.IntSizeInBytes
	JetTerminateJobCodecRequestInitialFrameSize          = JetTerminateJobCodecRequestLightJobCoordinatorOffset + proto.UuidSizeInBytes
)

func EncodeJetTerminateJobRequest(job codecTypes.JetTerminateJob) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, JetTerminateJobCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeLong(initialFrame.Content, JetTerminateJobCodecRequestJobIdOffset, job.JobId)
	EncodeInt(initialFrame.Content, JetTerminateJobCodecRequestTerminateModeOffset, int32(job.TerminateMode))
	EncodeUUID(initialFrame.Content, JetTerminateJobCodecRequestLightJobCoordinatorOffset, job.LightJobCoordinator)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(JetTerminateJobCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	return clientMessage
}
