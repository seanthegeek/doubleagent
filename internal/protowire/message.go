package protowire

import (
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
