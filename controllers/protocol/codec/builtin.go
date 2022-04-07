package codec

import (
	"encoding/binary"

	iserialization "github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
)

// Encoder for ClientMessage and value
type Encoder func(message *proto.ClientMessage, value interface{})

// Decoder creates iserialization.Data
type Decoder func(frameIterator *proto.ForwardFrameIterator) iserialization.Data

func DecodeNullableForString(frameIterator *proto.ForwardFrameIterator) string {
	if NextFrameIsNullFrame(frameIterator) {
		return ""
	}
	return DecodeString(frameIterator)
}

func NextFrameIsNullFrame(frameIterator *proto.ForwardFrameIterator) bool {
	isNullFrame := frameIterator.PeekNext().IsNullFrame()
	if isNullFrame {
		frameIterator.Next()
	}
	return isNullFrame
}

func DecodeString(frameIterator *proto.ForwardFrameIterator) string {
	return string(frameIterator.Next().Content)
}

func EncodeLong(buffer []byte, offset int32, value int64) {
	binary.LittleEndian.PutUint64(buffer[offset:], uint64(value))
}

func EncodeString(message *proto.ClientMessage, value interface{}) {
	message.AddFrame(proto.NewFrame([]byte(value.(string))))
}

func EncodeData(message *proto.ClientMessage, value interface{}) {
	message.AddFrame(proto.NewFrame(value.(iserialization.Data).ToByteArray()))
}
