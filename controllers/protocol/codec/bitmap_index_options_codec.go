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

	types "github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
)

const (
	BitmapIndexOptionsCodecUniqueKeyTransformationFieldOffset      = 0
	BitmapIndexOptionsCodecUniqueKeyTransformationInitialFrameSize = BitmapIndexOptionsCodecUniqueKeyTransformationFieldOffset + proto.IntSizeInBytes
)

func EncodeBitmapIndexOptions(clientMessage *proto.ClientMessage, bitmapIndexOptions types.BitmapIndexOptions) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, BitmapIndexOptionsCodecUniqueKeyTransformationInitialFrameSize))
	EncodeInt(initialFrame.Content, BitmapIndexOptionsCodecUniqueKeyTransformationFieldOffset, int32(bitmapIndexOptions.UniqueKeyTransformation))
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, bitmapIndexOptions.UniqueKey)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableForBitmapIndexOptions(message *proto.ClientMessage, options types.BitmapIndexOptions) {
	if options == (types.BitmapIndexOptions{}) {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeBitmapIndexOptions(message, options)
	}
}

func DecodeBitmapIndexOptions(frameIterator *proto.ForwardFrameIterator) types.BitmapIndexOptions {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	uniqueKeyTransformation := DecodeInt(initialFrame.Content, BitmapIndexOptionsCodecUniqueKeyTransformationFieldOffset)

	uniqueKey := DecodeString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.BitmapIndexOptions{
		UniqueKey:               uniqueKey,
		UniqueKeyTransformation: types.UniqueKeyTransformation(uniqueKeyTransformation),
	}
}

//manual
func DecodeNullableForBitmapIndexOptions(frameIterator *proto.ForwardFrameIterator) types.BitmapIndexOptions {
	if NextFrameIsNullFrame(frameIterator) {
		return types.BitmapIndexOptions{}
	}
	return DecodeBitmapIndexOptions(frameIterator)
}
