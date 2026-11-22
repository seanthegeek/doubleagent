package protowire

import (
	"math/big"
	"slices"
)

// Entry is one decoded field: its name and value. A value is one of string,
// []byte, *big.Int, bool, float64, *Message, or []any for a repeated field
// (holding values of those types). A timestamp is a string.
type Entry struct {
	Name  string
	Value any
}

// Message is a decoded message: its fields in the order they first appeared
// on the wire. A plain field seen twice keeps its first position and takes the
// last value; a repeated field collects every occurrence.
type Message struct {
	entries []Entry
}

// Entries returns the decoded fields in order.
func (m *Message) Entries() []Entry {
	return slices.Clone(m.entries)
}

// Get returns the value of a field and whether the message has it.
func (m *Message) Get(name string) (any, bool) {
	for _, entry := range m.entries {
		if entry.Name == name {
			return entry.Value, true
		}
	}
	return nil, false
}

// Text returns a string field's value, or "" and false when the field is
// absent or holds another type.
func (m *Message) Text(name string) (string, bool) {
	value, ok := m.Get(name)
	text, isText := value.(string)
	return text, ok && isText
}

// Int returns an integer field's value when the field is present and fits an
// int64.
func (m *Message) Int(name string) (int64, bool) {
	value, _ := m.Get(name)
	number, ok := value.(*big.Int)
	if !ok || !number.IsInt64() {
		return 0, false
	}
	return number.Int64(), true
}

// Child returns a nested message field.
func (m *Message) Child(name string) (*Message, bool) {
	value, _ := m.Get(name)
	child, ok := value.(*Message)
	return child, ok
}

// List returns a repeated field's values, or nil when the field is absent.
func (m *Message) List(name string) []any {
	value, _ := m.Get(name)
	list, _ := value.([]any)
	return list
}

// set stores a plain field, replacing the value of an earlier one in place.
func (m *Message) set(name string, value any) {
	for i := range m.entries {
		if m.entries[i].Name == name {
			m.entries[i].Value = value
			return
		}
	}
	m.entries = append(m.entries, Entry{name, value})
}

// extend appends values to a repeated field, creating it (possibly empty).
func (m *Message) extend(name string, values []any) {
	for i := range m.entries {
		if m.entries[i].Name == name {
			list, _ := m.entries[i].Value.([]any)
			m.entries[i].Value = append(list, values...)
			return
		}
	}
	m.entries = append(m.entries, Entry{name, append([]any{}, values...)})
}
