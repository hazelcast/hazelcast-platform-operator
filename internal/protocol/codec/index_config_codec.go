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

	types "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	IndexConfigCodecTypeFieldOffset      = 0
	IndexConfigCodecTypeInitialFrameSize = IndexConfigCodecTypeFieldOffset + proto.IntSizeInBytes
)

func EncodeIndexConfig(clientMessage *proto.ClientMessage, indexConfig types.IndexConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, IndexConfigCodecTypeInitialFrameSize))
	EncodeInt(initialFrame.Content, IndexConfigCodecTypeFieldOffset, int32(indexConfig.Type))
	clientMessage.AddFrame(initialFrame)

	EncodeNullableForString(clientMessage, indexConfig.Name)
	EncodeListMultiFrameForString(clientMessage, indexConfig.Attributes)
	EncodeNullableForBitmapIndexOptions(clientMessage, indexConfig.BitmapIndexOptions) //Before 	EncodeNullableForBitmapIndexOptions(clientMessage, &indexConfig.BitmapIndexOptions)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeListMultiFrameIndexConfig(message *proto.ClientMessage, values []types.IndexConfig) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(values); i++ {
		EncodeIndexConfig(message, values[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableListMultiFrameForIndexConfig(message *proto.ClientMessage, values []types.IndexConfig) {
	if values == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeListMultiFrameIndexConfig(message, values)
	}
}

func DecodeIndexConfig(frameIterator *proto.ForwardFrameIterator) types.IndexConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	_type := DecodeInt(initialFrame.Content, IndexConfigCodecTypeFieldOffset)

	name := DecodeNullableForString(frameIterator)
	attributes := DecodeListMultiFrameForString(frameIterator)
	bitmapIndexOptions := DecodeNullableForBitmapIndexOptions(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.IndexConfig{
		Name:               name,
		Type:               _type,
		Attributes:         attributes,
		BitmapIndexOptions: bitmapIndexOptions,
	}
}

//manual
func DecodeNullableListMultiFrameForIndexConfig(frameIterator *proto.ForwardFrameIterator) []types.IndexConfig {
	if NextFrameIsNullFrame(frameIterator) {
		return nil
	}
	var ic []types.IndexConfig
	DecodeListMultiFrame(frameIterator, func(it *proto.ForwardFrameIterator) {
		ic = append(ic, DecodeIndexConfig(it))
	})
	return ic
}
