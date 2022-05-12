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
	QueryCacheConfigHolderCodecBatchSizeFieldOffset     = 0
	QueryCacheConfigHolderCodecBufferSizeFieldOffset    = QueryCacheConfigHolderCodecBatchSizeFieldOffset + proto.IntSizeInBytes
	QueryCacheConfigHolderCodecDelaySecondsFieldOffset  = QueryCacheConfigHolderCodecBufferSizeFieldOffset + proto.IntSizeInBytes
	QueryCacheConfigHolderCodecIncludeValueFieldOffset  = QueryCacheConfigHolderCodecDelaySecondsFieldOffset + proto.IntSizeInBytes
	QueryCacheConfigHolderCodecPopulateFieldOffset      = QueryCacheConfigHolderCodecIncludeValueFieldOffset + proto.BooleanSizeInBytes
	QueryCacheConfigHolderCodecCoalesceFieldOffset      = QueryCacheConfigHolderCodecPopulateFieldOffset + proto.BooleanSizeInBytes
	QueryCacheConfigHolderCodecCoalesceInitialFrameSize = QueryCacheConfigHolderCodecCoalesceFieldOffset + proto.BooleanSizeInBytes
)

func EncodeQueryCacheConfigHolder(clientMessage *proto.ClientMessage, queryCacheConfigHolder types.QueryCacheConfigHolder) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, QueryCacheConfigHolderCodecCoalesceInitialFrameSize))
	EncodeInt(initialFrame.Content, QueryCacheConfigHolderCodecBatchSizeFieldOffset, int32(queryCacheConfigHolder.BatchSize))
	EncodeInt(initialFrame.Content, QueryCacheConfigHolderCodecBufferSizeFieldOffset, int32(queryCacheConfigHolder.BufferSize))
	EncodeInt(initialFrame.Content, QueryCacheConfigHolderCodecDelaySecondsFieldOffset, int32(queryCacheConfigHolder.DelaySeconds))
	EncodeBoolean(initialFrame.Content, QueryCacheConfigHolderCodecIncludeValueFieldOffset, queryCacheConfigHolder.IncludeValue)
	EncodeBoolean(initialFrame.Content, QueryCacheConfigHolderCodecPopulateFieldOffset, queryCacheConfigHolder.Populate)
	EncodeBoolean(initialFrame.Content, QueryCacheConfigHolderCodecCoalesceFieldOffset, queryCacheConfigHolder.Coalesce)
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, queryCacheConfigHolder.InMemoryFormat)
	EncodeString(clientMessage, queryCacheConfigHolder.Name)
	EncodePredicateConfigHolder(clientMessage, queryCacheConfigHolder.PredicateConfigHolder)
	EncodeEvictionConfigHolder(clientMessage, queryCacheConfigHolder.EvictionConfigHolder)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, queryCacheConfigHolder.ListenerConfigs)
	EncodeNullableListMultiFrameForIndexConfig(clientMessage, queryCacheConfigHolder.IndexConfigs)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeListMultiFrameQueryCacheConfigHolder(message *proto.ClientMessage, values []types.QueryCacheConfigHolder) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(values); i++ {
		EncodeQueryCacheConfigHolder(message, values[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableListMultiFrameForQueryCacheConfigHolder(message *proto.ClientMessage, values []types.QueryCacheConfigHolder) {
	if values == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeListMultiFrameQueryCacheConfigHolder(message, values)
	}
}

func DecodeQueryCacheConfigHolder(frameIterator *proto.ForwardFrameIterator) types.QueryCacheConfigHolder {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	batchSize := DecodeInt(initialFrame.Content, QueryCacheConfigHolderCodecBatchSizeFieldOffset)
	bufferSize := DecodeInt(initialFrame.Content, QueryCacheConfigHolderCodecBufferSizeFieldOffset)
	delaySeconds := DecodeInt(initialFrame.Content, QueryCacheConfigHolderCodecDelaySecondsFieldOffset)
	includeValue := DecodeBoolean(initialFrame.Content, QueryCacheConfigHolderCodecIncludeValueFieldOffset)
	populate := DecodeBoolean(initialFrame.Content, QueryCacheConfigHolderCodecPopulateFieldOffset)
	coalesce := DecodeBoolean(initialFrame.Content, QueryCacheConfigHolderCodecCoalesceFieldOffset)

	inMemoryFormat := DecodeString(frameIterator)
	name := DecodeString(frameIterator)
	predicateConfigHolder := DecodePredicateConfigHolder(frameIterator)
	evictionConfigHolder := DecodeEvictionConfigHolder(frameIterator)
	listenerConfigs := DecodeNullableListMultiFrameForListenerConfigHolder(frameIterator)
	indexConfigs := DecodeNullableListMultiFrameForIndexConfig(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.QueryCacheConfigHolder{
		BatchSize:             batchSize,
		BufferSize:            bufferSize,
		DelaySeconds:          delaySeconds,
		IncludeValue:          includeValue,
		Populate:              populate,
		Coalesce:              coalesce,
		InMemoryFormat:        inMemoryFormat,
		Name:                  name,
		PredicateConfigHolder: predicateConfigHolder,
		EvictionConfigHolder:  evictionConfigHolder,
		ListenerConfigs:       listenerConfigs,
		IndexConfigs:          indexConfigs,
	}
}
