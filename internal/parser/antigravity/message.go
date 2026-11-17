package antigravity

import (
	"math/big"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/protowire"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// message is a decoded protobuf message read as the Python analyzer reads
// its dict: an absent message is empty, and a missing field gives the
// zero of its use.
type message struct {
	decoded *protowire.Message
}

// field returns a field's value and whether the message has it.
func (m message) field(name string) (any, bool) {
	if m.decoded == nil {
		return nil, false
	}
	return m.decoded.Get(name)
}

// has reports whether the message has the field.
func (m message) has(name string) bool {
	_, ok := m.field(name)
	return ok
}

// text returns a field as Python's str() writes it, "" when it is absent.
func (m message) text(name string) string {
	value, ok := m.field(name)
	if !ok {
		return ""
	}
	return valueText(value)
}

// truthy reports whether a field is present and Python counts its value
// as true.
func (m message) truthy(name string) bool {
	value, _ := m.field(name)
	return isTruthy(value)
}

// child returns a nested message field, empty when it is absent.
func (m message) child(name string) message {
	value, _ := m.field(name)
	nested, _ := value.(*protowire.Message)
	return message{nested}
}

// list returns a repeated field's values, none when it is absent.
func (m message) list(name string) []any {
	value, _ := m.field(name)
	values, _ := value.([]any)
	return values
}

// children returns a repeated message field's messages.
func (m message) children(name string) []message {
	var out []message
	for _, value := range m.list(name) {
		nested, _ := value.(*protowire.Message)
		out = append(out, message{nested})
	}
	return out
}

// empty reports whether the message holds no field, which Python counts
// as false.
func (m message) empty() bool {
	return m.decoded == nil || len(m.decoded.Entries()) == 0
}

// first returns the text of the first of the fields Python counts as true,
// as `a or b or ""` does, else "".
func (m message) first(names ...string) string {
	for _, name := range names {
		if m.truthy(name) {
			return m.text(name)
		}
	}
	return ""
}

// isTruthy reports whether Python counts a decoded value as true.
func isTruthy(value any) bool {
	switch value := value.(type) {
	case string:
		return value != ""
	case []byte:
		return len(value) > 0
	case *big.Int:
		return value.Sign() != 0
	case bool:
		return value
	case float64:
		return value != 0
	case *protowire.Message:
		return value != nil && len(value.Entries()) > 0
	case []any:
		return len(value) > 0
	}
	return false
}

// valueText returns a decoded value as Python's str() writes it.
func valueText(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case *big.Int:
		return value.String()
	case bool:
		return pytext.Bool(value)
	case float64:
		return pytext.Float(value)
	}
	return jsonValue(value).Text()
}

// intValue returns an integer field as an int64, false when it is absent,
// not an integer or too large.
func (m message) intValue(name string) (int64, bool) {
	value, _ := m.field(name)
	number, ok := value.(*big.Int)
	if !ok || !number.IsInt64() {
		return 0, false
	}
	return number.Int64(), true
}

// jsonValue returns a decoded value as the JSON value Python's json module
// writes for it.
func jsonValue(value any) jsonvalue.Value {
	switch value := value.(type) {
	case string:
		return jsonvalue.NewString(value)
	case *big.Int:
		number, _ := jsonvalue.DecodeString(value.String())
		return number
	case bool:
		return jsonvalue.NewBool(value)
	case float64:
		return jsonvalue.NewFloat(value)
	case []byte:
		return jsonvalue.NewString(pytext.DecodeUTF8(value))
	case []any:
		items := make([]jsonvalue.Value, len(value))
		for i, item := range value {
			items[i] = jsonValue(item)
		}
		return jsonvalue.NewArray(items...)
	case *protowire.Message:
		var members []jsonvalue.Member
		if value == nil {
			return jsonvalue.NewObject()
		}
		for _, entry := range value.Entries() {
			members = append(members, jsonvalue.Member{Key: entry.Name, Value: jsonValue(entry.Value)})
		}
		return jsonvalue.NewObject(members...)
	}
	return jsonvalue.NewNull()
}

// join returns the non-empty parts joined by " | ".
func join(parts ...string) string {
	kept := parts[:0:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " | ")
}

// boolText returns "true" or "false" for a field's truth.
func boolText(fields message, name string) string {
	if fields.truthy(name) {
		return "true"
	}
	return "false"
}
