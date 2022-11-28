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
	MapStoreConfigHolderCodecEnabledFieldOffset             = 0
	MapStoreConfigHolderCodecWriteCoalescingFieldOffset     = MapStoreConfigHolderCodecEnabledFieldOffset + proto.BooleanSizeInBytes
	MapStoreConfigHolderCodecWriteDelaySecondsFieldOffset   = MapStoreConfigHolderCodecWriteCoalescingFieldOffset + proto.BooleanSizeInBytes
	MapStoreConfigHolderCodecWriteBatchSizeFieldOffset      = MapStoreConfigHolderCodecWriteDelaySecondsFieldOffset + proto.IntSizeInBytes
	MapStoreConfigHolderCodecWriteBatchSizeInitialFrameSize = MapStoreConfigHolderCodecWriteBatchSizeFieldOffset + proto.IntSizeInBytes
)

func EncodeMapStoreConfigHolder(clientMessage *proto.ClientMessage, mapStoreConfigHolder types.MapStoreConfigHolder) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, MapStoreConfigHolderCodecWriteBatchSizeInitialFrameSize))
	EncodeBoolean(initialFrame.Content, MapStoreConfigHolderCodecEnabledFieldOffset, mapStoreConfigHolder.Enabled)
	EncodeBoolean(initialFrame.Content, MapStoreConfigHolderCodecWriteCoalescingFieldOffset, mapStoreConfigHolder.WriteCoalescing)
	EncodeInt(initialFrame.Content, MapStoreConfigHolderCodecWriteDelaySecondsFieldOffset, int32(mapStoreConfigHolder.WriteDelaySeconds))
	EncodeInt(initialFrame.Content, MapStoreConfigHolderCodecWriteBatchSizeFieldOffset, int32(mapStoreConfigHolder.WriteBatchSize))
	clientMessage.AddFrame(initialFrame)

	EncodeNullableForString(clientMessage, mapStoreConfigHolder.ClassName)
	EncodeNullableForData(clientMessage, mapStoreConfigHolder.Implementation)
	EncodeNullableForString(clientMessage, mapStoreConfigHolder.FactoryClassName)
	EncodeNullableForData(clientMessage, mapStoreConfigHolder.FactoryImplementation)
	EncodeNullableMapForStringAndString(clientMessage, mapStoreConfigHolder.Properties)
	EncodeString(clientMessage, mapStoreConfigHolder.InitialLoadMode)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

// manual
func EncodeNullableForMapStoreConfigHolder(clientMessage *proto.ClientMessage, mapStoreConfigHolder types.MapStoreConfigHolder) {
	// types.MapStoreConfigHolder{} is not comparable with ==
	if reflect.DeepEqual(types.MapStoreConfigHolder{}, mapStoreConfigHolder) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeMapStoreConfigHolder(clientMessage, mapStoreConfigHolder)
	}
}

func DecodeMapStoreConfigHolder(frameIterator *proto.ForwardFrameIterator) types.MapStoreConfigHolder {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	enabled := DecodeBoolean(initialFrame.Content, MapStoreConfigHolderCodecEnabledFieldOffset)
	writeCoalescing := DecodeBoolean(initialFrame.Content, MapStoreConfigHolderCodecWriteCoalescingFieldOffset)
	writeDelaySeconds := DecodeInt(initialFrame.Content, MapStoreConfigHolderCodecWriteDelaySecondsFieldOffset)
	writeBatchSize := DecodeInt(initialFrame.Content, MapStoreConfigHolderCodecWriteBatchSizeFieldOffset)

	className := DecodeNullableForString(frameIterator)
	implementation := DecodeNullableForData(frameIterator)
	factoryClassName := DecodeNullableForString(frameIterator)
	factoryImplementation := DecodeNullableForData(frameIterator)
	properties := DecodeNullableMapForStringAndString(frameIterator)
	initialLoadMode := DecodeString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.MapStoreConfigHolder{
		Enabled:               enabled,
		WriteCoalescing:       writeCoalescing,
		WriteDelaySeconds:     writeDelaySeconds,
		WriteBatchSize:        writeBatchSize,
		ClassName:             className,
		Implementation:        implementation,
		FactoryClassName:      factoryClassName,
		FactoryImplementation: factoryImplementation,
		Properties:            properties,
		InitialLoadMode:       initialLoadMode,
	}
}
