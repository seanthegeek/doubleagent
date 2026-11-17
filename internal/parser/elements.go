package parser

import (
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// Elements returns what a Python loop over `value or []` visits: the items
// of an array, the keys of an object and the characters of a string, each as
// a string value, and nothing for null, false, zero or an empty container.
// A number or true is not iterable in Python, so it gives nothing here. A
// call site reads a visited item with Get, which gives null on anything but
// an object, as `_dict(item).get(key)` does.
func Elements(value jsonvalue.Value) []jsonvalue.Value {
	switch {
	case value.IsEmpty():
		return nil
	case value.IsArray():
		return value.Items()
	case value.IsObject():
		keys := value.Keys()
		items := make([]jsonvalue.Value, len(keys))
		for i, key := range keys {
			items[i] = jsonvalue.NewString(key)
		}
		return items
	case value.IsString():
		var items []jsonvalue.Value
		for rest := value.StringValue(); rest != ""; {
			_, size := utf8.DecodeRuneInString(rest)
			items = append(items, jsonvalue.NewString(rest[:size]))
			rest = rest[size:]
		}
		return items
	}
	return nil
}
