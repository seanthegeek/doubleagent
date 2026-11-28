package protowire

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// errMalformed is wrapped by every error that says the bytes are not a
// well-formed message.
var errMalformed = errors.New("malformed protobuf")

const (
	wireVarint     = 0
	wireFixed64    = 1
	wireLength     = 2
	wireStartGroup = 3
	wireEndGroup   = 4
	wireFixed32    = 5
)

// rawField is one field as it sits on the wire. A varint field has its
// number in varint; a fixed-width or length-delimited field has its bytes
// in data, and a group none. cut marks a fixed-width field that the end of
// its buffer cuts short.
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
		return 0, offset, fmt.Errorf("%w: truncated varint at offset %d", errMalformed, offset)
	case length < 0:
		return 0, offset, fmt.Errorf("%w: varint too long at offset %d", errMalformed, offset)
	}
	return value, offset + length, nil
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
		return rawField{}, offset, fmt.Errorf("%w: truncated length-delimited field at offset %d", errMalformed, offset)
	}
	field.data = data[offset : offset+int(length)]
	return field, offset + int(length), nil
}

// readField reads the field at offset. A fixed-width field cut short by the
// end of the buffer is marked cut, which leaves it out of the message and
// is no error, since nothing follows it; a cut tag, varint or length is. A
// group is read to its end tag (skipGroup), and an end tag with no group is
// an error.
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
	case wireStartGroup:
		offset, err = skipGroup(data, offset, field.number)
		return field, offset, err
	case wireEndGroup:
		return rawField{}, offset, fmt.Errorf("%w: end of group %d with no start", errMalformed, field.number)
	}
	return rawField{}, offset, fmt.Errorf("%w: unsupported wire type %d", errMalformed, field.wire)
}

// skipGroup returns the offset after the end tag of the group of number
// whose start tag ends at offset, passing over the fields inside it and
// the groups nested in it, at most maxDepth deep. A group with no end, or
// an end tag of another number than the group it closes, is an error.
func skipGroup(data []byte, offset int, number uint64) (int, error) {
	open := []uint64{number}
	for len(open) > 0 {
		if offset == len(data) {
			return offset, fmt.Errorf("%w: group %d has no end", errMalformed, open[len(open)-1])
		}
		tag, next, err := readVarint(data, offset)
		if err != nil {
			return next, err
		}
		switch tag & 7 {
		case wireStartGroup:
			if len(open) == maxDepth {
				return next, fmt.Errorf("%w: groups nest too deeply", errMalformed)
			}
			open, offset = append(open, tag>>3), next
		case wireEndGroup:
			if tag>>3 != open[len(open)-1] {
				return next, fmt.Errorf("%w: group %d ends with the tag of %d", errMalformed, open[len(open)-1], tag>>3)
			}
			open, offset = open[:len(open)-1], next
		default:
			if _, offset, err = readField(data, offset); err != nil {
				return offset, err
			}
		}
	}
	return offset, nil
}
