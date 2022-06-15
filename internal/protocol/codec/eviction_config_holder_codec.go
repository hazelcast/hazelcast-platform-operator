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
	"reflect"

	proto "github.com/hazelcast/hazelcast-go-client"

	types "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	EvictionConfigHolderCodecSizeFieldOffset      = 0
	EvictionConfigHolderCodecSizeInitialFrameSize = EvictionConfigHolderCodecSizeFieldOffset + proto.IntSizeInBytes
)

func EncodeEvictionConfigHolder(clientMessage *proto.ClientMessage, evictionConfigHolder types.EvictionConfigHolder) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, EvictionConfigHolderCodecSizeInitialFrameSize))
	EncodeInt(initialFrame.Content, EvictionConfigHolderCodecSizeFieldOffset, int32(evictionConfigHolder.Size))
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, string(evictionConfigHolder.MaxSizePolicy))
	EncodeString(clientMessage, string(evictionConfigHolder.EvictionPolicy))
	EncodeNullableForString(clientMessage, evictionConfigHolder.ComparatorClassName)
	EncodeNullableForData(clientMessage, evictionConfigHolder.Comparator)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableForEvictionConfigHolder(clientMessage *proto.ClientMessage, evictionConfigHolder types.EvictionConfigHolder) {
	// types.EvictionConfigHolder{} is not comparable with ==
	if reflect.DeepEqual(types.EvictionConfigHolder{}, evictionConfigHolder) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeEvictionConfigHolder(clientMessage, evictionConfigHolder)
	}
}

func DecodeEvictionConfigHolder(frameIterator *proto.ForwardFrameIterator) types.EvictionConfigHolder {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	size := DecodeInt(initialFrame.Content, EvictionConfigHolderCodecSizeFieldOffset)

	maxSizePolicy := DecodeString(frameIterator)
	evictionPolicy := DecodeString(frameIterator)
	comparatorClassName := DecodeNullableForString(frameIterator)
	comparator := DecodeNullableForData(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.EvictionConfigHolder{
		Size:                size,
		MaxSizePolicy:       maxSizePolicy,
		EvictionPolicy:      evictionPolicy,
		ComparatorClassName: comparatorClassName,
		Comparator:          comparator,
	}
}
