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
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	DataPersistenceConfigCodecEnabledFieldOffset    = 0
	DataPersistenceConfigCodecFsyncFieldOffset      = DataPersistenceConfigCodecEnabledFieldOffset + proto.BooleanSizeInBytes
	DataPersistenceConfigCodecFsyncInitialFrameSize = DataPersistenceConfigCodecFsyncFieldOffset + proto.BooleanSizeInBytes
)

func EncodeDataPersistenceConfig(clientMessage *proto.ClientMessage, dataPersistenceConfig types.DataPersistenceConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, DataPersistenceConfigCodecFsyncInitialFrameSize))
	EncodeBoolean(initialFrame.Content, DataPersistenceConfigCodecEnabledFieldOffset, dataPersistenceConfig.Enabled)
	EncodeBoolean(initialFrame.Content, DataPersistenceConfigCodecFsyncFieldOffset, dataPersistenceConfig.Fsync)
	clientMessage.AddFrame(initialFrame)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func DecodeDataPersistenceConfig(frameIterator *proto.ForwardFrameIterator) types.DataPersistenceConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	enabled := DecodeBoolean(initialFrame.Content, DataPersistenceConfigCodecEnabledFieldOffset)
	fsync := DecodeBoolean(initialFrame.Content, DataPersistenceConfigCodecFsyncFieldOffset)
	FastForwardToEndFrame(frameIterator)

	return types.DataPersistenceConfig{
		Enabled: enabled,
		Fsync:   fsync,
	}
}
