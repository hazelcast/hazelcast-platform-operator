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
	WanReplicationRefCodecRepublishingEnabledFieldOffset      = 0
	WanReplicationRefCodecRepublishingEnabledInitialFrameSize = WanReplicationRefCodecRepublishingEnabledFieldOffset + proto.BooleanSizeInBytes
)

func EncodeWanReplicationRef(clientMessage *proto.ClientMessage, wanReplicationRef types.WanReplicationRef) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, WanReplicationRefCodecRepublishingEnabledInitialFrameSize))
	EncodeBoolean(initialFrame.Content, WanReplicationRefCodecRepublishingEnabledFieldOffset, wanReplicationRef.RepublishingEnabled)
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, wanReplicationRef.Name)
	EncodeString(clientMessage, wanReplicationRef.MergePolicyClassName)
	EncodeNullableListMultiFrameForString(clientMessage, wanReplicationRef.Filters)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableForWanReplicationRef(clientMessage *proto.ClientMessage, wanReplicationRef types.WanReplicationRef) {
	// types.WanReplicationRef{} is not comparable with ==
	if reflect.DeepEqual(types.WanReplicationRef{}, wanReplicationRef) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeWanReplicationRef(clientMessage, wanReplicationRef)
	}
}

func DecodeWanReplicationRef(frameIterator *proto.ForwardFrameIterator) types.WanReplicationRef {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	republishingEnabled := DecodeBoolean(initialFrame.Content, WanReplicationRefCodecRepublishingEnabledFieldOffset)

	name := DecodeString(frameIterator)
	mergePolicyClassName := DecodeString(frameIterator)
	filters := DecodeNullableListMultiFrameForString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.WanReplicationRef{
		Name:                 name,
		MergePolicyClassName: mergePolicyClassName,
		Filters:              filters,
		RepublishingEnabled:  republishingEnabled,
	}
}
