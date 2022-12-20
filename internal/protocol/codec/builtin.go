package codec

import (
	"encoding/binary"
	"math"
	"strings"

	iserialization "github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
)

// Encoder for ClientMessage and value
type Encoder func(message *proto.ClientMessage, value interface{})

// Decoder creates iserialization.Data
type Decoder func(frameIterator *proto.ForwardFrameIterator) iserialization.Data

func FastForwardToEndFrame(frameIterator *proto.ForwardFrameIterator) {
	expectedEndFrames := 1
	for expectedEndFrames != 0 {
		frame := frameIterator.Next()
		if frame.IsEndFrame() {
			expectedEndFrames--
		} else if frame.IsBeginFrame() {
			expectedEndFrames++
		}
	}
}

func EncodeNullable(message *proto.ClientMessage, value interface{}, encoder Encoder) {
	if value == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		encoder(message, value)
	}
}

func EncodeNullableForString(message *proto.ClientMessage, value string) {
	if strings.TrimSpace(value) == "" {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeString(message, value)
	}
}

func EncodeString(message *proto.ClientMessage, value interface{}) {
	message.AddFrame(proto.NewFrame([]byte(value.(string))))
}

func DecodeString(frameIterator *proto.ForwardFrameIterator) string {
	return string(frameIterator.Next().Content)
}

func EncodeBoolean(buffer []byte, offset int32, value bool) {
	if value {
		buffer[offset] = 1
	} else {
		buffer[offset] = 0
	}
}

func DecodeBoolean(buffer []byte, offset int32) bool {
	return buffer[offset] == 1
}

func EncodeByte(buffer []byte, offset int32, value byte) {
	buffer[offset] = value
}

func DecodeByte(buffer []byte, offset int32) byte {
	return buffer[offset]
}

func EncodeShort(buffer []byte, offset, value int32) {
	binary.LittleEndian.PutUint16(buffer[offset:], uint16(value))
}

func DecodeShort(buffer []byte, offset int32) int16 {
	return int16(binary.LittleEndian.Uint16(buffer[offset:]))
}

func EncodeInt(buffer []byte, offset, value int32) {
	binary.LittleEndian.PutUint32(buffer[offset:], uint32(value))
}

func DecodeInt(buffer []byte, offset int32) int32 {
	return int32(binary.LittleEndian.Uint32(buffer[offset:]))
}

func EncodeLong(buffer []byte, offset int32, value int64) {
	binary.LittleEndian.PutUint64(buffer[offset:], uint64(value))
}

func DecodeLong(buffer []byte, offset int32) int64 {
	return int64(binary.LittleEndian.Uint64(buffer[offset:]))
}

func EncodeFloat(buffer []byte, offset int32, value float32) {
	binary.LittleEndian.PutUint32(buffer[offset:], math.Float32bits(value))
}

func DecodeFloat(buffer []byte, offset int32) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(buffer[offset:]))
}

func EncodeDouble(buffer []byte, offset int32, value float64) {
	binary.LittleEndian.PutUint64(buffer[offset:], math.Float64bits(value))
}

func DecodeDouble(buffer []byte, offset int32) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(buffer[offset:]))
}

func EncodeNullableForData(message *proto.ClientMessage, data iserialization.Data) {
	if data == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeData(message, data)
	}
}

func DecodeNullableForData(frameIterator *proto.ForwardFrameIterator) iserialization.Data {
	if NextFrameIsNullFrame(frameIterator) {
		return iserialization.Data{}
	}
	return DecodeData(frameIterator)
}

func EncodeData(message *proto.ClientMessage, value interface{}) {
	message.AddFrame(proto.NewFrame(value.(iserialization.Data).ToByteArray()))
}

func EncodeNullableData(message *proto.ClientMessage, data iserialization.Data) {
	if data == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		message.AddFrame(proto.NewFrame(data.ToByteArray()))
	}
}

func DecodeData(frameIterator *proto.ForwardFrameIterator) iserialization.Data {
	return iserialization.Data(frameIterator.Next().Content)
}

func DecodeNullableData(frameIterator *proto.ForwardFrameIterator) iserialization.Data {
	if NextFrameIsNullFrame(frameIterator) {
		return iserialization.Data{}
	}
	return DecodeData(frameIterator)
}

func NextFrameIsNullFrame(frameIterator *proto.ForwardFrameIterator) bool {
	isNullFrame := frameIterator.PeekNext().IsNullFrame()
	if isNullFrame {
		frameIterator.Next()
	}
	return isNullFrame
}

func DecodeNullableForString(frameIterator *proto.ForwardFrameIterator) string {
	if NextFrameIsNullFrame(frameIterator) {
		return ""
	}
	return DecodeString(frameIterator)
}

func EncodeMapForStringAndString(message *proto.ClientMessage, values map[string]string) {
	message.AddFrame(proto.BeginFrame.Copy())
	for key, value := range values {
		EncodeString(message, key)
		EncodeString(message, value)
	}
	message.AddFrame(proto.EndFrame.Copy())
}
func EncodeNullableMapForStringAndString(message *proto.ClientMessage, values map[string]string) {
	if values == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeMapForStringAndString(message, values)
	}
}

func DecodeMapForStringAndString(iterator *proto.ForwardFrameIterator) map[string]string {
	result := map[string]string{}
	iterator.Next()
	for !iterator.PeekNext().IsEndFrame() {
		key := DecodeString(iterator)
		value := DecodeString(iterator)
		result[key] = value
	}
	iterator.Next()
	return result
}

func DecodeNullableMapForStringAndString(frameIterator *proto.ForwardFrameIterator) map[string]string {
	if NextFrameIsNullFrame(frameIterator) {
		return nil
	}
	return DecodeMapForStringAndString(frameIterator)
}

func EncodeListMultiFrameForString(message *proto.ClientMessage, values []string) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(values); i++ {
		EncodeString(message, values[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

func EncodeNullableListMultiFrameForString(message *proto.ClientMessage, values []string) {
	if values == nil {
		message.AddFrame(proto.NullFrame.Copy())
	} else {
		EncodeListMultiFrameForString(message, values)
	}
}

func DecodeListMultiFrameForString(frameIterator *proto.ForwardFrameIterator) []string {
	result := make([]string, 0)
	frameIterator.Next()
	for !NextFrameIsDataStructureEndFrame(frameIterator) {
		result = append(result, DecodeString(frameIterator))
	}
	frameIterator.Next()
	return result
}

func DecodeNullableListMultiFrameForString(frameIterator *proto.ForwardFrameIterator) []string {
	if NextFrameIsNullFrame(frameIterator) {
		return nil
	}
	return DecodeListMultiFrameForString(frameIterator)
}

func NextFrameIsDataStructureEndFrame(frameIterator *proto.ForwardFrameIterator) bool {
	return frameIterator.PeekNext().IsEndFrame()
}

func DecodeListMultiFrame(frameIterator *proto.ForwardFrameIterator, decoder func(frameIterator *proto.ForwardFrameIterator)) {
	frameIterator.Next()
	for !NextFrameIsDataStructureEndFrame(frameIterator) {
		decoder(frameIterator)
	}
	frameIterator.Next()
}

func EncodeListMultiFrameForData(message *proto.ClientMessage, values []iserialization.Data) {
	message.AddFrame(proto.BeginFrame.Copy())
	for i := 0; i < len(values); i++ {
		EncodeData(message, values[i])
	}
	message.AddFrame(proto.EndFrame.Copy())
}

func DecodeEntryListForDataAndData(frameIterator *proto.ForwardFrameIterator) []proto.Pair {
	result := make([]proto.Pair, 0)
	frameIterator.Next()
	for NextFrameIsDataStructureEndFrame(frameIterator) {
		key := DecodeData(frameIterator)
		value := DecodeData(frameIterator)
		result = append(result, proto.NewPair(key, value))
	}
	frameIterator.Next()
	return result
}

func EncodeEntryListForDataAndData(message *proto.ClientMessage, entries []proto.Pair) {
	message.AddFrame(proto.BeginFrame.Copy())
	for _, value := range entries {
		EncodeData(message, value.Key)
		EncodeData(message, value.Value)
	}
	message.AddFrame(proto.EndFrame.Copy())
}

func DecodeListMultiFrameForDataContainsNullable(frameIterator *proto.ForwardFrameIterator) []iserialization.Data {
	result := make([]iserialization.Data, 0)
	frameIterator.Next()
	for NextFrameIsDataStructureEndFrame(frameIterator) {
		if NextFrameIsNullFrame(frameIterator) {
			result = append(result, nil)
		} else {
			result = append(result, DecodeData(frameIterator))
		}
	}
	frameIterator.Next()
	return result
}
