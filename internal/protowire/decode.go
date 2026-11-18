package protowire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// maxDepth bounds how deeply messages nest; deeper input is left out as
// malformed, so a self-referencing schema cannot exhaust the stack.
const maxDepth = 100

// maxPackedValues is the most packed integers and booleans one Decode call
// reads, over all its fields. An agent's lists of this kind hold thousands
// of values at most, and each value costs about 70 bytes once decoded, so
// the bound keeps a message's packed values to about 70 MiB however large
// its buffer.
const maxPackedValues = 1 << 20

// ErrTooLarge is wrapped by the error of a message holding more packed
// values than one Decode call reads.
var ErrTooLarge = errors.New("protobuf message is larger than the limit")

// Decode reads data as the named message. Unknown fields are skipped. A
// field that does not fit its declared kind, a fixed-width field cut short
// by the end of its buffer, a packed field holding a damaged varint and a
// nested message that is malformed are each left out, the rest kept. The
// error wraps ErrMalformed when the top-level buffer cannot be followed (a
// tag, varint or length cut off, or an unsupported wire type), wraps
// ErrTooLarge past maxPackedValues, and is another error when the schema
// has no such message.
func (s *Schema) Decode(data []byte, message string) (*Message, error) {
	spec := s.messages[message]
	if spec == nil {
		return nil, fmt.Errorf("protowire: unknown message %q", message)
	}
	run := &decoding{schema: s}
	return run.message(data, spec, 0)
}

// decoding is the state of one Decode call: the schema and the packed
// values read so far.
type decoding struct {
	schema *Schema
	packed int
}

// message decodes data as the message spec describes, depth levels below
// the top; it fails where Decode says the top-level buffer fails.
func (d *decoding) message(data []byte, spec *messageSpec, depth int) (*Message, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("%w: messages nest too deeply", ErrMalformed)
	}
	message := &Message{}
	offset := 0
	for offset < len(data) {
		field, next, err := readField(data, offset)
		if err != nil {
			return nil, err
		}
		offset = next
		known, ok := spec.byNumber[field.number]
		if !ok || field.cut {
			continue
		}
		if err := d.addField(message, known, field, depth); err != nil {
			return nil, err
		}
	}
	return message, nil
}

// addField stores one known field in the message, or leaves it out when it
// cannot be read as its kind.
func (d *decoding) addField(message *Message, known fieldSpec, field rawField, depth int) error {
	if !isScalar(known.kind) {
		child, err := d.nested(known.kind, field, depth)
		if err != nil || child == nil {
			return err
		}
		store(message, known, child)
		return nil
	}
	if known.repeated && field.wire == wireLength && (known.kind == KindInt || known.kind == KindBool) {
		values, err := d.packedVarints(field.data, known.kind == KindBool)
		if isMalformed(err) {
			return nil
		}
		if err != nil {
			return err
		}
		message.extend(known.name, values)
		return nil
	}
	value, ok := scalar(known.kind, field)
	if ok {
		store(message, known, value)
	}
	return nil
}

// store sets a plain field's value, or adds one to a repeated field's list.
func store(message *Message, known fieldSpec, value any) {
	if known.repeated {
		message.extend(known.name, []any{value})
		return
	}
	message.set(known.name, value)
}

// nested decodes a nested message, returning nil when the field is not bytes
// or the bytes are malformed.
func (d *decoding) nested(kind string, field rawField, depth int) (*Message, error) {
	if field.wire == wireVarint {
		return nil, nil
	}
	child, err := d.message(field.data, d.schema.messages[kind], depth+1)
	if err != nil {
		if isMalformed(err) {
			return nil, nil
		}
		return nil, err
	}
	return child, nil
}

// packedVarints reads a packed run of varints as integers or booleans. A
// damaged varint is an ErrMalformed error, and the run passing
// maxPackedValues for the call an ErrTooLarge one.
func (d *decoding) packedVarints(data []byte, boolean bool) ([]any, error) {
	values := []any{}
	for offset := 0; offset < len(data); {
		item, next, err := readVarint(data, offset)
		if err != nil {
			return nil, err
		}
		if d.packed++; d.packed > maxPackedValues {
			return nil, fmt.Errorf("%w: more than %d packed values", ErrTooLarge, maxPackedValues)
		}
		offset = next
		if boolean {
			values = append(values, item != 0)
		} else {
			values = append(values, new(big.Int).SetUint64(item))
		}
	}
	return values, nil
}

// scalar converts a field to a scalar kind and says whether it could. A
// varint field is read as the number it holds, and a length-delimited or
// fixed-width field as its bytes.
func scalar(kind string, field rawField) (any, bool) {
	isBytes := field.wire != wireVarint
	switch kind {
	case KindString:
		if isBytes {
			return textlines.ValidUTF8(field.data), true
		}
		return new(big.Int).SetUint64(field.varint).String(), true
	case KindBytes:
		if isBytes {
			return bytes.Clone(field.data), true
		}
		return []byte{}, true
	case KindInt:
		return intScalar(field, isBytes)
	case KindBool:
		if isBytes {
			return len(field.data) > 0, true
		}
		return field.varint != 0, true
	case KindFloat, KindDouble:
		return floatScalar(kind, field, isBytes)
	}
	return timestampScalar(field, isBytes)
}

// intScalar reads a fixed-width little-endian signed integer from bytes of
// width 8 or 4, or a varint as unsigned.
func intScalar(field rawField, isBytes bool) (any, bool) {
	if !isBytes {
		return new(big.Int).SetUint64(field.varint), true
	}
	switch len(field.data) {
	case 8:
		return big.NewInt(int64(binary.LittleEndian.Uint64(field.data))), true
	case 4:
		return big.NewInt(int64(int32(binary.LittleEndian.Uint32(field.data)))), true
	}
	return nil, false
}

// floatScalar reads 4 bytes as a float32 and 8 bytes as a float64 according to
// the kind; a varint is converted to a float.
func floatScalar(kind string, field rawField, isBytes bool) (any, bool) {
	if !isBytes {
		return float64(field.varint), true
	}
	if kind == KindFloat && len(field.data) == 4 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(field.data))), true
	}
	if kind == KindDouble && len(field.data) == 8 {
		return math.Float64frombits(binary.LittleEndian.Uint64(field.data)), true
	}
	return nil, false
}

// timestampScalar decodes a Timestamp message, or a varint as epoch time;
// either form holding zero seconds holds no time and gives "".
func timestampScalar(field rawField, isBytes bool) (any, bool) {
	if !isBytes {
		if field.varint == 0 || field.varint > math.MaxInt64 {
			return "", true
		}
		return timeutil.FromInt(int64(field.varint)), true
	}
	text, err := timestampText(field.data)
	return text, err == nil
}

// timestampText decodes the seconds and nanoseconds of a Timestamp message
// as a timeline timestamp; no seconds, or seconds the varint holds as a
// negative int64, give "".
func timestampText(data []byte) (string, error) {
	var seconds, nanos uint64
	for offset := 0; offset < len(data); {
		field, next, err := readField(data, offset)
		if err != nil {
			return "", err
		}
		offset = next
		if field.wire != wireVarint {
			continue
		}
		switch field.number {
		case 1:
			seconds = field.varint
		case 2:
			nanos = field.varint
		}
	}
	if seconds == 0 || seconds > math.MaxInt64 || nanos > math.MaxInt64 {
		return "", nil
	}
	return timeutil.FromTime(time.Unix(int64(seconds), int64(nanos))), nil
}

// isMalformed reports whether err says the bytes are not a valid message.
func isMalformed(err error) bool {
	return errors.Is(err, ErrMalformed)
}
