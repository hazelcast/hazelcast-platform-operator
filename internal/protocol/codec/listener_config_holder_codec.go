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
	ListenerConfigHolderCodecListenerTypeFieldOffset = 0
	ListenerConfigHolderCodecIncludeValueFieldOffset = ListenerConfigHolderCodecListenerTypeFieldOffset + proto.IntSizeInBytes
	ListenerConfigHolderCodecLocalFieldOffset        = ListenerConfigHolderCodecIncludeValueFieldOffset + proto.BooleanSizeInBytes
	ListenerConfigHolderCodecLocalInitialFrameSize   = ListenerConfigHolderCodecLocalFieldOffset + proto.BooleanSizeInBytes
)

func EncodeListenerConfigHolder(clientMessage *proto.ClientMessage, listenerConfigHolder types.ListenerConfigHolder) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, ListenerConfigHolderCodecLocalInitialFrameSize))
	EncodeInt(initialFrame.Content, ListenerConfigHolderCodecListenerTypeFieldOffset, int32(listenerConfigHolder.ListenerType))
	EncodeBoolean(initialFrame.Content, ListenerConfigHolderCodecIncludeValueFieldOffset, listenerConfigHolder.IncludeValue)
	EncodeBoolean(initialFrame.Content, ListenerConfigHolderCodecLocalFieldOffset, listenerConfigHolder.Local)
	clientMessage.AddFrame(initialFrame)

	EncodeNullableForData(clientMessage, listenerConfigHolder.ListenerImplementation)
	EncodeNullableForString(clientMessage, listenerConfigHolder.ClassName)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeListMultiFrameListenerConfigHolder(message *proto.ClientMessage, values []types.ListenerConfigHolder) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(values); i++ {
		EncodeListenerConfigHolder(message, values[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableListMultiFrameForListenerConfigHolder(message *proto.ClientMessage, values []types.ListenerConfigHolder) {
	if values == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeListMultiFrameListenerConfigHolder(message, values)
	}
}

func DecodeListenerConfigHolder(frameIterator *proto.ForwardFrameIterator) types.ListenerConfigHolder {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	listenerType := DecodeInt(initialFrame.Content, ListenerConfigHolderCodecListenerTypeFieldOffset)
	includeValue := DecodeBoolean(initialFrame.Content, ListenerConfigHolderCodecIncludeValueFieldOffset)
	local := DecodeBoolean(initialFrame.Content, ListenerConfigHolderCodecLocalFieldOffset)

	listenerImplementation := DecodeNullableForData(frameIterator)
	className := DecodeNullableForString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.ListenerConfigHolder{
		ListenerType:           listenerType,
		ListenerImplementation: listenerImplementation,
		ClassName:              className,
		IncludeValue:           includeValue,
		Local:                  local,
	}
}

//manual
func DecodeNullableListMultiFrameForListenerConfigHolder(frameIterator *proto.ForwardFrameIterator) []types.ListenerConfigHolder {
	if NextFrameIsNullFrame(frameIterator) {
		return nil
	}
	var lch []types.ListenerConfigHolder
	DecodeListMultiFrame(frameIterator, func(it *proto.ForwardFrameIterator) {
		lch = append(lch, DecodeListenerConfigHolder(it))
	})
	return lch
}
