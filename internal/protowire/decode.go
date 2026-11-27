package protowire

import (
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

// maxValues is the most values one Decode call reads, nested messages
// included: every scalar, packed value and nested message counts once,
// whether its field is repeated or not. Decoding a value allocates up to
// about 150 bytes, beyond the bytes of a string, which come from the input,
// so the bound keeps a call to about 150 MiB however large its buffer. The
// longest list an agent writes is the step indices of an Antigravity
// conversation, one per step; the conversations studied held at most 125.
const maxValues = 1 << 20

// ErrTooLarge is wrapped by the error of a message holding more values
// than one Decode call reads.
var ErrTooLarge = errors.New("protobuf message is larger than the limit")

// Decode reads data as the named message. Unknown fields, and fields
// whose wire type does not fit their declared kind, are skipped. A field
// that cannot be read in its declared kind, a fixed-width field cut short
// by the end of its buffer, a packed field holding a damaged varint and a
// nested message that is malformed are each left out, the rest kept. The
// error wraps ErrMalformed when the top-level buffer cannot be followed (a
// tag, varint or length cut off, or an unsupported wire type), wraps
// ErrTooLarge when the message holds more than 1,048,576 values, counting
// each scalar, packed value and nested message at any depth, and is
// another error when the schema has no such message.
func (s *Schema) Decode(data []byte, message string) (*Message, error) {
	spec := s.messages[message]
	if spec == nil {
		return nil, fmt.Errorf("protowire: unknown message %q", message)
	}
	run := &decoding{schema: s}
	return run.message(data, spec, 0)
}

// decoding is the state of one Decode call: the schema and how many
// values it has read so far.
type decoding struct {
	schema *Schema
	values int
}

// count adds one value to the call's total, refusing one past maxValues.
func (d *decoding) count() error {
	if d.values++; d.values > maxValues {
		return fmt.Errorf("%w: more than %d values", ErrTooLarge, maxValues)
	}
	return nil
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

// addField stores one known field in the message, or leaves it out when
// its wire type does not fit its kind or it cannot be read as its kind.
func (d *decoding) addField(message *Message, known fieldSpec, field rawField, depth int) error {
	packed := known.repeated && field.wire == wireLength && (known.kind == KindInt || known.kind == KindBool)
	if !packed && !wireFits(known.kind, field.wire) {
		return nil
	}
	if !isScalar(known.kind) {
		if err := d.count(); err != nil {
			return err
		}
		child, err := d.nested(known.kind, field, depth)
		if err != nil || child == nil {
			return err
		}
		store(message, known, child)
		return nil
	}
	if packed {
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
	if err := d.count(); err != nil {
		return err
	}
	if value, ok := scalar(known.kind, field); ok {
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

// nested decodes a nested message, returning nil when its bytes are
// malformed.
func (d *decoding) nested(kind string, field rawField, depth int) (*Message, error) {
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
// damaged varint is an ErrMalformed error, and a value past maxValues for
// the call an ErrTooLarge one.
func (d *decoding) packedVarints(data []byte, boolean bool) ([]any, error) {
	values := []any{}
	for offset := 0; offset < len(data); {
		item, next, err := readVarint(data, offset)
		if err != nil {
			return nil, err
		}
		if err := d.count(); err != nil {
			return nil, err
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

// wireFits reports whether a field's wire type is the one its kind is
// written with: a varint or a fixed width for an integer, a varint for a
// truth value, and length-delimited bytes for text, a timestamp and a
// nested message. A protobuf reader treats a field of another wire type as
// unknown.
func wireFits(kind string, wire uint64) bool {
	switch kind {
	case KindInt:
		return wire == wireVarint || wire == wireFixed64 || wire == wireFixed32
	case KindBool:
		return wire == wireVarint
	}
	return wire == wireLength
}

// scalar converts a field whose wire type fits its scalar kind and says
// whether it could: a fixed-width integer is little-endian and signed, a
// varint integer unsigned, and a malformed timestamp cannot be read.
func scalar(kind string, field rawField) (any, bool) {
	switch kind {
	case KindString:
		return textlines.ValidUTF8(field.data), true
	case KindBool:
		return field.varint != 0, true
	case KindTimestamp:
		text, err := timestampText(field.data)
		return text, err == nil
	}
	switch field.wire {
	case wireFixed64:
		return big.NewInt(int64(binary.LittleEndian.Uint64(field.data))), true
	case wireFixed32:
		return big.NewInt(int64(int32(binary.LittleEndian.Uint32(field.data)))), true
	}
	return new(big.Int).SetUint64(field.varint), true
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
