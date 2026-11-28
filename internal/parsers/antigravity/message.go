package antigravity

import (
	"math/big"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/protowire"
)

// message is a decoded protobuf message: an absent message is empty, and a
// missing field gives the zero of its use (no text, not set, no items).
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

// jsonValue returns a decoded value as a JSON value, a message as an object
// in field order.
func jsonValue(value any) jsonvalue.Value {
	switch value := value.(type) {
	case string:
		return jsonvalue.NewString(value)
	case *big.Int:
		number, _ := jsonvalue.DecodeString(value.String())
		return number
	case bool:
		return jsonvalue.NewBool(value)
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

// valueText returns a decoded value as row text: a string as it is, an
// integer in decimal, true or false, and lists and messages as their JSON
// (jsonValue).
func valueText(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case *big.Int:
		return value.String()
	case bool:
		return strconv.FormatBool(value)
	}
	return jsonValue(value).Text()
}

// text returns a field as row text (valueText), "" when it is absent.
func (m message) text(name string) string {
	value, ok := m.field(name)
	if !ok {
		return ""
	}
	return valueText(value)
}

// isNonEmpty reports whether a decoded value is not empty by DoubleAgent's
// rule (jsonvalue.Value.IsEmpty): a non-empty string or list, a non-zero
// integer, true, or a message with a field.
func isNonEmpty(value any) bool {
	switch value := value.(type) {
	case string:
		return value != ""
	case *big.Int:
		return value.Sign() != 0
	case bool:
		return value
	case *protowire.Message:
		return value != nil && len(value.Entries()) > 0
	case []any:
		return len(value) > 0
	}
	return false
}

// set reports whether a field is present and not empty (isNonEmpty).
func (m message) set(name string) bool {
	value, _ := m.field(name)
	return isNonEmpty(value)
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

// empty reports whether the message holds no field.
func (m message) empty() bool {
	return m.decoded == nil || len(m.decoded.Entries()) == 0
}

// first returns the text of the first of the fields that is set, else "".
func (m message) first(names ...string) string {
	for _, name := range names {
		if m.set(name) {
			return m.text(name)
		}
	}
	return ""
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

// boolText returns "true" or "false" for a field's truth.
func boolText(fields message, name string) string {
	if fields.set(name) {
		return "true"
	}
	return "false"
}
