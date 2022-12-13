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
	HotRestartConfigCodecEnabledFieldOffset    = 0
	HotRestartConfigCodecFsyncFieldOffset      = HotRestartConfigCodecEnabledFieldOffset + proto.BooleanSizeInBytes
	HotRestartConfigCodecFsyncInitialFrameSize = HotRestartConfigCodecFsyncFieldOffset + proto.BooleanSizeInBytes
)

func EncodeHotRestartConfig(clientMessage *proto.ClientMessage, hotRestartConfig types.HotRestartConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, HotRestartConfigCodecFsyncInitialFrameSize))
	EncodeBoolean(initialFrame.Content, HotRestartConfigCodecEnabledFieldOffset, hotRestartConfig.Enabled)
	EncodeBoolean(initialFrame.Content, HotRestartConfigCodecFsyncFieldOffset, hotRestartConfig.Fsync)
	clientMessage.AddFrame(initialFrame)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

// manual
func EncodeNullableForHotRestartConfig(clientMessage *proto.ClientMessage, hotRestartConfig types.HotRestartConfig) {
	if hotRestartConfig == (types.HotRestartConfig{}) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeHotRestartConfig(clientMessage, hotRestartConfig)
	}
}

func DecodeHotRestartConfig(frameIterator *proto.ForwardFrameIterator) types.HotRestartConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	enabled := DecodeBoolean(initialFrame.Content, HotRestartConfigCodecEnabledFieldOffset)
	fsync := DecodeBoolean(initialFrame.Content, HotRestartConfigCodecFsyncFieldOffset)
	FastForwardToEndFrame(frameIterator)

	return types.HotRestartConfig{
		Enabled: enabled,
		Fsync:   fsync,
	}
}
