package protowire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrMalformed is wrapped by every error that says the bytes are not a
// well-formed message.
var ErrMalformed = errors.New("malformed protobuf")

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireLength  = 2
	wireFixed32 = 5
)

// rawField is one field as it sits on the wire. A varint field has its
// number in varint; every other wire type has its bytes in data. cut marks
// a fixed-width field that the end of its buffer cuts short.
type rawField struct {
	number uint64
	wire   uint64
	varint uint64
	data   []byte
	cut    bool
}

// readVarint returns the varint at offset and the offset after it. A varint
// that the end of data cuts off, or that needs more than 64 bits, is an
// error.
func readVarint(data []byte, offset int) (uint64, int, error) {
	value, length := binary.Uvarint(data[offset:])
	switch {
	case length == 0:
		return 0, offset, fmt.Errorf("%w: truncated varint at offset %d", ErrMalformed, offset)
	case length < 0:
		return 0, offset, fmt.Errorf("%w: varint too long at offset %d", ErrMalformed, offset)
	}
	return value, offset + length, nil
}

// readField reads the field at offset. A fixed-width field cut short by the
// end of the buffer is marked cut, which leaves it out of the message and
// is no error, since nothing follows it; a cut tag, varint or length is.
func readField(data []byte, offset int) (rawField, int, error) {
	tag, offset, err := readVarint(data, offset)
	if err != nil {
		return rawField{}, offset, err
	}
	field := rawField{number: tag >> 3, wire: tag & 7}
	switch field.wire {
	case wireVarint:
		field.varint, offset, err = readVarint(data, offset)
		return field, offset, err
	case wireFixed64:
		field.data, field.cut, offset = fixedBytes(data, offset, 8)
		return field, offset, nil
	case wireFixed32:
		field.data, field.cut, offset = fixedBytes(data, offset, 4)
		return field, offset, nil
	case wireLength:
		return readLengthDelimited(field, data, offset)
	}
	return rawField{}, offset, fmt.Errorf("%w: unsupported wire type %d", ErrMalformed, field.wire)
}

// fixedBytes returns the width bytes at offset, whether the end of data
// cuts them short, and the offset after them.
func fixedBytes(data []byte, offset, width int) ([]byte, bool, int) {
	if width > len(data)-offset {
		return nil, true, len(data)
	}
	return data[offset : offset+width], false, offset + width
}

// readLengthDelimited reads the length and the bytes it counts.
func readLengthDelimited(field rawField, data []byte, offset int) (rawField, int, error) {
	length, offset, err := readVarint(data, offset)
	if err != nil {
		return rawField{}, offset, err
	}
	if length > uint64(len(data)-offset) {
		return rawField{}, offset, fmt.Errorf("%w: truncated length-delimited field at offset %d", ErrMalformed, offset)
	}
	field.data = data[offset : offset+int(length)]
	return field, offset + int(length), nil
}
