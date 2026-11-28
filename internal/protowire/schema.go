package protowire

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// The scalar kinds, as a Field's Kind spells them. Any other kind names a
// nested message of the schema.
const (
	// kindString is UTF-8 text; invalid bytes become U+FFFD.
	kindString = "str"
	// KindInt is an integer: a varint is unsigned, a fixed width is signed.
	KindInt = "int"
	// kindBool is a truth value.
	kindBool = "bool"
	// kindTimestamp is a google.protobuf.Timestamp, decoded to
	// `YYYY-MM-DDTHH:MM:SS.mmmZ` or "" when it holds no time.
	kindTimestamp = "ts"
)

// Field describes one field of a message. A Name that starts with "+" marks a
// repeated field, which decodes to a list under the name without the "+".
// Packed repeated integers and booleans are read too.
type Field struct {
	Number uint64
	Name   string
	Kind   string
}

// Schema is a checked set of message descriptions.
type Schema struct {
	messages map[string]*messageSpec
}

type fieldSpec struct {
	name     string
	kind     string
	repeated bool
}

type messageSpec struct {
	order    []Field
	byNumber map[uint64]fieldSpec
}

// isScalar reports whether kind is one of the scalar kinds rather than the
// name of a nested message.
func isScalar(kind string) bool {
	switch kind {
	case kindString, KindInt, kindBool, kindTimestamp:
		return true
	}
	return false
}

// newMessageSpec indexes one message and checks its fields against each other.
func newMessageSpec(name string, fields []Field) (*messageSpec, error) {
	spec := &messageSpec{order: fields, byNumber: map[uint64]fieldSpec{}}
	repeatedByName := map[string]bool{}
	for _, field := range fields {
		if _, seen := spec.byNumber[field.Number]; seen {
			return nil, fmt.Errorf("protowire: message %s: field number %d used twice", name, field.Number)
		}
		bare := strings.TrimPrefix(field.Name, "+")
		repeated := bare != field.Name
		if was, seen := repeatedByName[bare]; seen && was != repeated {
			return nil, fmt.Errorf("protowire: message %s: field name %q is both repeated and plain", name, bare)
		}
		repeatedByName[bare] = repeated
		spec.byNumber[field.Number] = fieldSpec{name: bare, kind: field.Kind, repeated: repeated}
	}
	return spec, nil
}

// NewSchema checks and indexes the messages, each given as its fields in
// declaration order. It fails on a nested kind that names no message, a field
// number used twice in one message, and a name that is repeated in one field
// and plain in another of the same message.
func NewSchema(messages map[string][]Field) (*Schema, error) {
	schema := &Schema{messages: map[string]*messageSpec{}}
	names := make([]string, 0, len(messages))
	for name := range messages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec, err := newMessageSpec(name, messages[name])
		if err != nil {
			return nil, err
		}
		schema.messages[name] = spec
	}
	for _, name := range names {
		for _, field := range messages[name] {
			if !isScalar(field.Kind) && schema.messages[field.Kind] == nil {
				return nil, fmt.Errorf("protowire: message %s field %d: unknown message %q", name, field.Number, field.Kind)
			}
		}
	}
	return schema, nil
}

// Fields returns the fields of a message in the order they were declared, or
// nil when the schema has no such message.
func (s *Schema) Fields(message string) []Field {
	spec := s.messages[message]
	if spec == nil {
		return nil
	}
	return slices.Clone(spec.order)
}
