package codec

import (
	proto "github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

const (
	DynamicConfigAddMultiMapConfigCodecRequestMessageType  = int32(0x1B0100)
	DynamicConfigAddMultiMapConfigCodecResponseMessageType = int32(0x1B0101)

	DynamicConfigAddMultiMapConfigCodecRequestBinaryOffset            = proto.PartitionIDOffset + proto.IntSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestBackupCountOffset       = DynamicConfigAddMultiMapConfigCodecRequestBinaryOffset + proto.BooleanSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestAsyncBackupCountOffset  = DynamicConfigAddMultiMapConfigCodecRequestBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestStatisticsEnabledOffset = DynamicConfigAddMultiMapConfigCodecRequestAsyncBackupCountOffset + proto.IntSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestMergeBatchSizeOffset    = DynamicConfigAddMultiMapConfigCodecRequestStatisticsEnabledOffset + proto.BooleanSizeInBytes
	DynamicConfigAddMultiMapConfigCodecRequestInitialFrameSize        = DynamicConfigAddMultiMapConfigCodecRequestMergeBatchSizeOffset + proto.IntSizeInBytes
)

// Adds a new multimap config to a running cluster.
// If a multimap configuration with the given {@code name} already exists, then
// the new multimap config is ignored and the existing one is preserved.

func EncodeDynamicConfigAddMultiMapConfigRequest(c *types.MultiMapConfig) *proto.ClientMessage {
	clientMessage := proto.NewClientMessageForEncode()
	clientMessage.SetRetryable(false)

	initialFrame := proto.NewFrameWith(make([]byte, DynamicConfigAddMultiMapConfigCodecRequestInitialFrameSize), proto.UnfragmentedMessage)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestBinaryOffset, c.Binary)
	EncodeInt(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestBackupCountOffset, c.BackupCount)
	EncodeInt(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestAsyncBackupCountOffset, c.AsyncBackupCount)
	EncodeBoolean(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestStatisticsEnabledOffset, c.StatisticsEnabled)
	EncodeInt(initialFrame.Content, DynamicConfigAddMultiMapConfigCodecRequestMergeBatchSizeOffset, c.MergeBatchSize)
	clientMessage.AddFrame(initialFrame)
	clientMessage.SetMessageType(DynamicConfigAddMultiMapConfigCodecRequestMessageType)
	clientMessage.SetPartitionId(-1)

	EncodeString(clientMessage, c.Name)
	EncodeString(clientMessage, c.CollectionType)
	EncodeNullableListMultiFrameForListenerConfigHolder(clientMessage, c.ListenerConfigs)
	EncodeNullableForString(clientMessage, c.SplitBrainProtectionName)
	EncodeString(clientMessage, c.MergePolicy)

	return clientMessage
}
