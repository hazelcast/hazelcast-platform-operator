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

	"github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
)

const (
	NearCachePreloaderConfigCodecEnabledFieldOffset                   = 0
	NearCachePreloaderConfigCodecStoreInitialDelaySecondsFieldOffset  = NearCachePreloaderConfigCodecEnabledFieldOffset + proto.BooleanSizeInBytes
	NearCachePreloaderConfigCodecStoreIntervalSecondsFieldOffset      = NearCachePreloaderConfigCodecStoreInitialDelaySecondsFieldOffset + proto.IntSizeInBytes
	NearCachePreloaderConfigCodecStoreIntervalSecondsInitialFrameSize = NearCachePreloaderConfigCodecStoreIntervalSecondsFieldOffset + proto.IntSizeInBytes
)

func EncodeNearCachePreloaderConfig(clientMessage *proto.ClientMessage, nearCachePreloaderConfig types.NearCachePreloaderConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, NearCachePreloaderConfigCodecStoreIntervalSecondsInitialFrameSize))
	EncodeBoolean(initialFrame.Content, NearCachePreloaderConfigCodecEnabledFieldOffset, nearCachePreloaderConfig.Enabled)
	EncodeInt(initialFrame.Content, NearCachePreloaderConfigCodecStoreInitialDelaySecondsFieldOffset, int32(nearCachePreloaderConfig.StoreInitialDelaySeconds))
	EncodeInt(initialFrame.Content, NearCachePreloaderConfigCodecStoreIntervalSecondsFieldOffset, int32(nearCachePreloaderConfig.StoreIntervalSeconds))
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, nearCachePreloaderConfig.Directory)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableForNearCachePreloaderConfig(clientMessage *proto.ClientMessage, nearCachePreloaderConfig types.NearCachePreloaderConfig) {
	if nearCachePreloaderConfig == (types.NearCachePreloaderConfig{}) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeNearCachePreloaderConfig(clientMessage, nearCachePreloaderConfig)
	}
}

func DecodeNearCachePreloaderConfig(frameIterator *proto.ForwardFrameIterator) types.NearCachePreloaderConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	enabled := DecodeBoolean(initialFrame.Content, NearCachePreloaderConfigCodecEnabledFieldOffset)
	storeInitialDelaySeconds := DecodeInt(initialFrame.Content, NearCachePreloaderConfigCodecStoreInitialDelaySecondsFieldOffset)
	storeIntervalSeconds := DecodeInt(initialFrame.Content, NearCachePreloaderConfigCodecStoreIntervalSecondsFieldOffset)

	directory := DecodeString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.NearCachePreloaderConfig{
		Enabled:                  enabled,
		Directory:                directory,
		StoreInitialDelaySeconds: storeInitialDelaySeconds,
		StoreIntervalSeconds:     storeIntervalSeconds,
	}
}

//manual
func DecodeNullableForNearCachePreloaderConfig(frameIterator *proto.ForwardFrameIterator) types.NearCachePreloaderConfig {
	if NextFrameIsNullFrame(frameIterator) {
		return types.NearCachePreloaderConfig{}
	}
	return DecodeNearCachePreloaderConfig(frameIterator)
}
