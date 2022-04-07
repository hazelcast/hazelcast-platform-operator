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
	MCGetMapConfigCodecRequestMessageType  = int32(0x200300)
	MCGetMapConfigCodecResponseMessageType = int32(0x200301)

	MCGetMapConfigCodecRequestInitialFrameSize = proto.PartitionIDOffset + proto.IntSizeInBytes

	MCGetMapConfigResponseInMemoryFormatOffset    = proto.ResponseBackupAcksOffset + proto.ByteSizeInBytes
	MCGetMapConfigResponseBackupCountOffset       = MCGetMapConfigResponseInMemoryFormatOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseAsyncBackupCountOffset  = MCGetMapConfigResponseBackupCountOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseTimeToLiveSecondsOffset = MCGetMapConfigResponseAsyncBackupCountOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseMaxIdleSecondsOffset    = MCGetMapConfigResponseTimeToLiveSecondsOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseMaxSizeOffset           = MCGetMapConfigResponseMaxIdleSecondsOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseMaxSizePolicyOffset     = MCGetMapConfigResponseMaxSizeOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseReadBackupDataOffset    = MCGetMapConfigResponseMaxSizePolicyOffset + proto.IntSizeInBytes
	MCGetMapConfigResponseEvictionPolicyOffset    = MCGetMapConfigResponseReadBackupDataOffset + proto.BooleanSizeInBytes
)

// Gets the config of a map on the member it's called on.

func EncodeMCGetMapConfigRequest(mapName string) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(true)

	initialFrame := proto.NewFrameWith(make([]byte, MCGetMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCGetMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, mapName)

	return clientMessage
}

func DecodeMCGetMapConfigResponse(clientMessage *proto.ClientMessage) MapConfig {
	frameIterator := clientMessage.FrameIterator()
	initialFrame := frameIterator.Next()
	return MapConfig{
		InMemoryFormat:    DecodeInt(initialFrame.Content, MCGetMapConfigResponseInMemoryFormatOffset),
		BackupCount:       DecodeInt(initialFrame.Content, MCGetMapConfigResponseBackupCountOffset),
		AsyncBackupCount:  DecodeInt(initialFrame.Content, MCGetMapConfigResponseAsyncBackupCountOffset),
		TimeToLiveSeconds: DecodeInt(initialFrame.Content, MCGetMapConfigResponseTimeToLiveSecondsOffset),
		MaxIdleSeconds:    DecodeInt(initialFrame.Content, MCGetMapConfigResponseMaxIdleSecondsOffset),
		MaxSize:           DecodeInt(initialFrame.Content, MCGetMapConfigResponseMaxSizeOffset),
		MaxSizePolicy:     DecodeInt(initialFrame.Content, MCGetMapConfigResponseMaxSizePolicyOffset),
		ReadBackupData:    DecodeBoolean(initialFrame.Content, MCGetMapConfigResponseReadBackupDataOffset),
		EvictionPolicy:    DecodeInt(initialFrame.Content, MCGetMapConfigResponseEvictionPolicyOffset),
		MergePolicy:       DecodeString(frameIterator),
	}

}

type MapConfig struct {
	InMemoryFormat    int32
	BackupCount       int32
	AsyncBackupCount  int32
	TimeToLiveSeconds int32
	MaxIdleSeconds    int32
	MaxSize           int32
	MaxSizePolicy     int32
	ReadBackupData    bool
	EvictionPolicy    int32
	MergePolicy       string
}
