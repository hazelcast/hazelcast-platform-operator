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

	types "github.com/hazelcast/hazelcast-platform-operator/controllers/protocol/types"
)

func EncodeAttributeConfig(clientMessage *proto.ClientMessage, attributeConfig types.AttributeConfig) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())

	EncodeString(clientMessage, attributeConfig.Name)
	EncodeString(clientMessage, attributeConfig.ExtractorClassName)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeListMultiFrameAttributeConfig(message *proto.ClientMessage, values []types.AttributeConfig) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(values); i++ {
		EncodeAttributeConfig(message, values[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

//manual
func EncodeNullableListMultiFrameForAttributeConfig(message *proto.ClientMessage, values []types.AttributeConfig) {
	if values == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeListMultiFrameAttributeConfig(message, values)
	}
}

func DecodeAttributeConfig(frameIterator *proto.ForwardFrameIterator) types.AttributeConfig {
	// begin frame
	frameIterator.Next()

	name := DecodeString(frameIterator)
	extractorClassName := DecodeString(frameIterator)
	FastForwardToEndFrame(frameIterator)

	return types.AttributeConfig{
		Name:               name,
		ExtractorClassName: extractorClassName,
	}
}
