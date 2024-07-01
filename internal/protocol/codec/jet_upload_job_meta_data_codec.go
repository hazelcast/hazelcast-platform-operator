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

	types "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	JetUploadJobMetaDataCodecRequestMessageType  = int32(0xFE1100)
	JetUploadJobMetaDataCodecResponseMessageType = int32(0xFE1101)

	JetUploadJobMetaDataCodecRequestSessionIdOffset   = proto.PartitionIDOffset + proto.IntSizeInBytes
	JetUploadJobMetaDataCodecRequestJarOnMemberOffset = JetUploadJobMetaDataCodecRequestSessionIdOffset + proto.UuidSizeInBytes
	JetUploadJobMetaDataCodecRequestInitialFrameSize  = JetUploadJobMetaDataCodecRequestJarOnMemberOffset + proto.BooleanSizeInBytes
)

func EncodeJetUploadJobMetaDataRequest(jobMetaData types.JobMetaData) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(true)

	initialFrame := proto.NewFrameWith(make([]byte, JetUploadJobMetaDataCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeUUID(initialFrame.Content, JetUploadJobMetaDataCodecRequestSessionIdOffset, jobMetaData.SessionId)
	EncodeBoolean(initialFrame.Content, JetUploadJobMetaDataCodecRequestJarOnMemberOffset, jobMetaData.JarOnMember)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(JetUploadJobMetaDataCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, jobMetaData.FileName)
	EncodeString(clientMessage, jobMetaData.Sha256Hex)
	EncodeNullableForString(clientMessage, jobMetaData.SnapshotName)
	EncodeNullableForString(clientMessage, jobMetaData.JobName)
	EncodeNullableForString(clientMessage, jobMetaData.MainClass)
	EncodeListMultiFrameForString(clientMessage, jobMetaData.JobParameters)

	return clientMessage
}

func DecodeJetUploadJobMetaDataRequest(clientMessage *proto.ClientMessage) types.JobMetaData {
	jobMetaData := types.JobMetaData{}

	frameIterator := clientMessage.FrameIterator()
	initialFrame := frameIterator.Next()

	jobMetaData.SessionId = DecodeUUID(initialFrame.Content, JetUploadJobMetaDataCodecRequestSessionIdOffset)
	jobMetaData.JarOnMember = DecodeBoolean(initialFrame.Content, JetUploadJobMetaDataCodecRequestJarOnMemberOffset)

	jobMetaData.FileName = DecodeString(frameIterator)
	jobMetaData.Sha256Hex = DecodeString(frameIterator)
	jobMetaData.SnapshotName = DecodeNullableForString(frameIterator)
	jobMetaData.JobName = DecodeNullableForString(frameIterator)
	jobMetaData.MainClass = DecodeNullableForString(frameIterator)
	jobMetaData.JobParameters = DecodeListMultiFrameForString(frameIterator)

	return jobMetaData
}
