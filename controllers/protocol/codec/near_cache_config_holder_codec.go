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

	types "github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
)

const (
	NearCacheConfigHolderCodecSerializeKeysFieldOffset          = 0
	NearCacheConfigHolderCodecInvalidateOnChangeFieldOffset     = NearCacheConfigHolderCodecSerializeKeysFieldOffset + proto.BooleanSizeInBytes
	NearCacheConfigHolderCodecTimeToLiveSecondsFieldOffset      = NearCacheConfigHolderCodecInvalidateOnChangeFieldOffset + proto.BooleanSizeInBytes
	NearCacheConfigHolderCodecMaxIdleSecondsFieldOffset         = NearCacheConfigHolderCodecTimeToLiveSecondsFieldOffset + proto.IntSizeInBytes
	NearCacheConfigHolderCodecCacheLocalEntriesFieldOffset      = NearCacheConfigHolderCodecMaxIdleSecondsFieldOffset + proto.IntSizeInBytes
	NearCacheConfigHolderCodecCacheLocalEntriesInitialFrameSize = NearCacheConfigHolderCodecCacheLocalEntriesFieldOffset + proto.BooleanSizeInBytes
)

func EncodeNearCacheConfigHolder(clientMessage *proto.ClientMessage, nearCacheConfigHolder types.NearCacheConfigHolder) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, NearCacheConfigHolderCodecCacheLocalEntriesInitialFrameSize))
	EncodeBoolean(initialFrame.Content, NearCacheConfigHolderCodecSerializeKeysFieldOffset, nearCacheConfigHolder.SerializeKeys)
	EncodeBoolean(initialFrame.Content, NearCacheConfigHolderCodecInvalidateOnChangeFieldOffset, nearCacheConfigHolder.InvalidateOnChange)
	EncodeInt(initialFrame.Content, NearCacheConfigHolderCodecTimeToLiveSecondsFieldOffset, int32(nearCacheConfigHolder.TimeToLiveSeconds))
	EncodeInt(initialFrame.Content, NearCacheConfigHolderCodecMaxIdleSecondsFieldOffset, int32(nearCacheConfigHolder.MaxIdleSeconds))
	EncodeBoolean(initialFrame.Content, NearCacheConfigHolderCodecCacheLocalEntriesFieldOffset, nearCacheConfigHolder.CacheLocalEntries)
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, nearCacheConfigHolder.Name)
	EncodeString(clientMessage, nearCacheConfigHolder.InMemoryFormat)
	EncodeEvictionConfigHolder(clientMessage, nearCacheConfigHolder.EvictionConfigHolder)
	EncodeString(clientMessage, nearCacheConfigHolder.LocalUpdatePolicy)
	EncodeNullableForNearCachePreloaderConfig(clientMessage, nearCacheConfigHolder.PreloaderConfig)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableForNearCacheConfigHolder(clientMessage *proto.ClientMessage, nearCacheConfigHolder types.NearCacheConfigHolder) {
	// types.NearCacheConfigHolder{} is not comparable with ==
	if reflect.DeepEqual(types.NearCacheConfigHolder{}, nearCacheConfigHolder) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeNearCacheConfigHolder(clientMessage, nearCacheConfigHolder)
	}
}

func DecodeNearCacheConfigHolder(frameIterator *proto.ForwardFrameIterator) types.NearCacheConfigHolder {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	serializeKeys := DecodeBoolean(initialFrame.Content, NearCacheConfigHolderCodecSerializeKeysFieldOffset)
	invalidateOnChange := DecodeBoolean(initialFrame.Content, NearCacheConfigHolderCodecInvalidateOnChangeFieldOffset)
	timeToLiveSeconds := DecodeInt(initialFrame.Content, NearCacheConfigHolderCodecTimeToLiveSecondsFieldOffset)
	maxIdleSeconds := DecodeInt(initialFrame.Content, NearCacheConfigHolderCodecMaxIdleSecondsFieldOffset)
	cacheLocalEntries := DecodeBoolean(initialFrame.Content, NearCacheConfigHolderCodecCacheLocalEntriesFieldOffset)

	name := DecodeString(frameIterator)
	inMemoryFormat := DecodeString(frameIterator)
	evictionConfigHolder := DecodeEvictionConfigHolder(frameIterator)
	localUpdatePolicy := DecodeString(frameIterator)
	preloaderConfig := DecodeNullableForNearCachePreloaderConfig(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.NearCacheConfigHolder{
		Name:                 name,
		InMemoryFormat:       inMemoryFormat,
		SerializeKeys:        serializeKeys,
		InvalidateOnChange:   invalidateOnChange,
		TimeToLiveSeconds:    timeToLiveSeconds,
		MaxIdleSeconds:       maxIdleSeconds,
		EvictionConfigHolder: evictionConfigHolder,
		CacheLocalEntries:    cacheLocalEntries,
		LocalUpdatePolicy:    localUpdatePolicy,
		PreloaderConfig:      preloaderConfig,
	}
}
