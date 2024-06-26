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
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"

	proto "github.com/hazelcast/hazelcast-go-client"
)

const (
	DynamicConfigAddUserCodeNamespaceConfigCodecRequestMessageType  = int32(0x1b1300)
	DynamicConfigAddUserCodeNamespaceConfigCodecResponseMessageType = int32(0x1b1301)

	DynamicConfigAddUserCodeNamespaceConfigCodecRequestInitialFrameSize = proto.PartitionIDOffset + proto.IntSizeInBytes
)

// Adds a user code namespace configuration.

func EncodeDynamicConfigAddUserCodeNamespaceConfigRequest(config *types.UserCodeNamespaceConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddUserCodeNamespaceConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddUserCodeNamespaceConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, config.Name)
	EncodeListMultiFrameResourceDefinition(clientMessage, config.Resources)

	return clientMessage
}
