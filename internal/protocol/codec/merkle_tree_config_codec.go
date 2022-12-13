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
	MerkleTreeConfigCodecEnabledFieldOffset         = 0
	MerkleTreeConfigCodecDepthFieldOffset           = MerkleTreeConfigCodecEnabledFieldOffset + proto.BooleanSizeInBytes
	MerkleTreeConfigCodecEnabledSetFieldOffset      = MerkleTreeConfigCodecDepthFieldOffset + proto.IntSizeInBytes
	MerkleTreeConfigCodecEnabledSetInitialFrameSize = MerkleTreeConfigCodecEnabledSetFieldOffset + proto.BooleanSizeInBytes
)

func EncodeMerkleTreeConfig(clientMessage *proto.ClientMessage, merkleTreeConfig types.MerkleTreeConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, MerkleTreeConfigCodecEnabledSetInitialFrameSize))
	EncodeBoolean(initialFrame.Content, MerkleTreeConfigCodecEnabledFieldOffset, merkleTreeConfig.Enabled)
	EncodeInt(initialFrame.Content, MerkleTreeConfigCodecDepthFieldOffset, int32(merkleTreeConfig.Depth))
	EncodeBoolean(initialFrame.Content, MerkleTreeConfigCodecEnabledSetFieldOffset, merkleTreeConfig.EnabledSet)
	clientMessage.AddFrame(initialFrame)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

// manual
func EncodeNullableForMerkleTreeConfig(clientMessage *proto.ClientMessage, merkleTreeConfig types.MerkleTreeConfig) {
	if merkleTreeConfig == (types.MerkleTreeConfig{}) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeMerkleTreeConfig(clientMessage, merkleTreeConfig)
	}
}

func DecodeMerkleTreeConfig(frameIterator *proto.ForwardFrameIterator) types.MerkleTreeConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	enabled := DecodeBoolean(initialFrame.Content, MerkleTreeConfigCodecEnabledFieldOffset)
	depth := DecodeInt(initialFrame.Content, MerkleTreeConfigCodecDepthFieldOffset)
	// let enabledSet = false	if (initialFrame.content.length >= ENABLED_SET_OFFSET + BitsUtil.BOOLEAN_SIZE_IN_BYTES) {
	// enabledSet = DecodeBoolean(initialFrame.Content, ENABLED_SET_OFFSET)
	// }
	enabledSet := false
	if len(initialFrame.Content) >= MerkleTreeConfigCodecEnabledSetFieldOffset+proto.BooleanSizeInBytes {
		enabledSet = DecodeBoolean(initialFrame.Content, MerkleTreeConfigCodecEnabledSetFieldOffset)
	}
	FastForwardToEndFrame(frameIterator)

	return types.MerkleTreeConfig{
		Enabled:    enabled,
		Depth:      depth,
		EnabledSet: enabledSet,
	}
}
