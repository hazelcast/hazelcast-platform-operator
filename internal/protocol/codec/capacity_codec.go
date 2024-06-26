/*
* Copyright (c) 2008-2023, Hazelcast, Inc. All Rights Reserved.
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

var (
	EncodeUnit = map[string]int32{
		"BYTES":     0,
		"KILOBYTES": 1,
		"MEGABYTES": 2,
		"GIGABYTES": 3,
	}
	DecodeUnit = map[int32]string{
		0: "BYTES",
		1: "KILOBYTES",
		2: "MEGABYTES",
		3: "GIGABYTES",
	}
)

const (
	CapacityCodecValueFieldOffset     = 0
	CapacityCodecUnitFieldOffset      = CapacityCodecValueFieldOffset + proto.LongSizeInBytes
	CapacityCodecUnitInitialFrameSize = CapacityCodecUnitFieldOffset + proto.IntSizeInBytes
)

func EncodeCapacity(clientMessage *proto.ClientMessage, capacity types.Capacity) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, CapacityCodecUnitInitialFrameSize))
	EncodeLong(initialFrame.Content, CapacityCodecValueFieldOffset, int64(capacity.Value))
	EncodeInt(initialFrame.Content, CapacityCodecUnitFieldOffset, EncodeUnit[capacity.Unit])
	clientMessage.AddFrame(initialFrame)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func DecodeCapacity(frameIterator *proto.ForwardFrameIterator) types.Capacity {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	value := DecodeLong(initialFrame.Content, CapacityCodecValueFieldOffset)
	unit := DecodeInt(initialFrame.Content, CapacityCodecUnitFieldOffset)
	FastForwardToEndFrame(frameIterator)

	return types.Capacity{
		Value: value,
		Unit:  DecodeUnit[unit],
	}
}
