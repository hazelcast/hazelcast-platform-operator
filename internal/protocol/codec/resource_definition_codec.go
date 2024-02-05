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
	ResourceDefinitionTypeFieldOffset = 0
)

func EncodeResourceDefinition(clientMessage *proto.ClientMessage, resource types.ResourceDefinition) {
	clientMessage.AddFrame(proto.BeginFrame.Copy())

	initialFrame := proto.NewFrame(make([]byte, ListenerConfigHolderCodecLocalInitialFrameSize))
	EncodeInt(initialFrame.Content, ResourceDefinitionTypeFieldOffset, int32(resource.ResourceType))
	clientMessage.AddFrame(initialFrame)

	EncodeString(clientMessage, resource.ID)
	EncodeNullableForByteArray(clientMessage, resource.Payload)
	EncodeNullableForString(clientMessage, resource.ResourceURL)

	clientMessage.AddFrame(proto.EndFrame.Copy())
}

func EncodeListMultiFrameResourceDefinition(message *proto.ClientMessage, resources []types.ResourceDefinition) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(resources); i++ {
		EncodeResourceDefinition(message, resources[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}
