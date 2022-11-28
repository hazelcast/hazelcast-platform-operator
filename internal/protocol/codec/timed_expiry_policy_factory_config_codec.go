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
	"reflect"
)

const (
	TimedExpiryPolicyFactoryConfigCodecExpiryPolicyTypeFieldOffset      = 0
	TimedExpiryPolicyFactoryConfigCodecExpiryPolicyTypeInitialFrameSize = TimedExpiryPolicyFactoryConfigCodecExpiryPolicyTypeFieldOffset + proto.IntSizeInBytes
)

//manual
func EncodeNullableForEncodeTimedExpiryPolicyFactoryConfig(clientMessage *proto.ClientMessage, timedExpiryPolicyFactoryConfig types.TimedExpiryPolicyFactoryConfig) {
	// types.EvictionConfigHolder{} is not comparable with ==
	if reflect.DeepEqual(types.TimedExpiryPolicyFactoryConfig{}, timedExpiryPolicyFactoryConfig) {
		clientMessage.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeTimedExpiryPolicyFactoryConfig(clientMessage, timedExpiryPolicyFactoryConfig)
	}
}

func EncodeTimedExpiryPolicyFactoryConfig(clientMessage *proto.ClientMessage, timedExpiryPolicyFactoryConfig types.TimedExpiryPolicyFactoryConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())
	initialFrame := proto.NewFrame(make([]byte, TimedExpiryPolicyFactoryConfigCodecExpiryPolicyTypeInitialFrameSize))
	EncodeInt(initialFrame.Content, TimedExpiryPolicyFactoryConfigCodecExpiryPolicyTypeFieldOffset, int32(timedExpiryPolicyFactoryConfig.ExpiryPolicyType))
	clientMessage.AddFrame(initialFrame)

	EncodeDurationConfig(clientMessage, timedExpiryPolicyFactoryConfig.DurationConfig)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func DecodeTimedExpiryPolicyFactoryConfig(frameIterator *proto.ForwardFrameIterator) types.TimedExpiryPolicyFactoryConfig {
	// begin frame
	frameIterator.Next()
	initialFrame := frameIterator.Next()
	expiryPolicyType := DecodeInt(initialFrame.Content, TimedExpiryPolicyFactoryConfigCodecExpiryPolicyTypeFieldOffset)

	durationConfig := DecodeDurationConfig(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.TimedExpiryPolicyFactoryConfig{
		ExpiryPolicyType: expiryPolicyType,
		DurationConfig:   durationConfig,
	}
}
