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
	MCAddWanBatchPublisherConfigCodecRequestMessageType  = int32(0x201500)
	MCAddWanBatchPublisherConfigCodecResponseMessageType = int32(0x201501)

	MCAddWanBatchPublisherConfigCodecRequestQueueCapacityOffset         = proto.PartitionIDOffset + proto.IntSizeInBytes
	MCAddWanBatchPublisherConfigCodecRequestBatchSizeOffset             = MCAddWanBatchPublisherConfigCodecRequestQueueCapacityOffset + proto.IntSizeInBytes
	MCAddWanBatchPublisherConfigCodecRequestBatchMaxDelayMillisOffset   = MCAddWanBatchPublisherConfigCodecRequestBatchSizeOffset + proto.IntSizeInBytes
	MCAddWanBatchPublisherConfigCodecRequestResponseTimeoutMillisOffset = MCAddWanBatchPublisherConfigCodecRequestBatchMaxDelayMillisOffset + proto.IntSizeInBytes
	MCAddWanBatchPublisherConfigCodecRequestAckTypeOffset               = MCAddWanBatchPublisherConfigCodecRequestResponseTimeoutMillisOffset + proto.IntSizeInBytes
	MCAddWanBatchPublisherConfigCodecRequestQueueFullBehaviorOffset     = MCAddWanBatchPublisherConfigCodecRequestAckTypeOffset + proto.IntSizeInBytes
	MCAddWanBatchPublisherConfigCodecRequestInitialFrameSize            = MCAddWanBatchPublisherConfigCodecRequestQueueFullBehaviorOffset + proto.IntSizeInBytes
)

// Add a new WAN batch publisher configuration

func EncodeMCAddWanBatchPublisherConfigRequest(name string, targetCluster string, publisherId string, endpoints string, queueCapacity int32, batchSize int32, batchMaxDelayMillis int32, responseTimeoutMillis int32, ackType int32, queueFullBehavior int32) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, MCAddWanBatchPublisherConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeInt(initialFrame.Content, MCAddWanBatchPublisherConfigCodecRequestQueueCapacityOffset, queueCapacity)
	EncodeInt(initialFrame.Content, MCAddWanBatchPublisherConfigCodecRequestBatchSizeOffset, batchSize)
	EncodeInt(initialFrame.Content, MCAddWanBatchPublisherConfigCodecRequestBatchMaxDelayMillisOffset, batchMaxDelayMillis)
	EncodeInt(initialFrame.Content, MCAddWanBatchPublisherConfigCodecRequestResponseTimeoutMillisOffset, responseTimeoutMillis)
	EncodeInt(initialFrame.Content, MCAddWanBatchPublisherConfigCodecRequestAckTypeOffset, ackType)
	EncodeInt(initialFrame.Content, MCAddWanBatchPublisherConfigCodecRequestQueueFullBehaviorOffset, queueFullBehavior)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(MCAddWanBatchPublisherConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, name)
	EncodeString(clientMessage, targetCluster)
	EncodeNullableForString(clientMessage, publisherId)
	EncodeString(clientMessage, endpoints)

	return clientMessage
}

func DecodeMCAddWanBatchPublisherConfigResponse(clientMessage *proto.ClientMessage) (addedPublisherIds []string, ignoredPublisherIds []string) {
	frameIterator := clientMessage.FrameIterator()
	frameIterator.Next()

	addedPublisherIds = DecodeListMultiFrameForString(frameIterator)
	ignoredPublisherIds = DecodeListMultiFrameForString(frameIterator)

	return addedPublisherIds, ignoredPublisherIds
}
