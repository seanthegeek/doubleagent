package parser

import (
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// Elements returns what a Python loop over `value or []` visits: the items
// of an array, the keys of an object and the characters of a string, each as
// a string value, and nothing for null, false, zero or an empty container.
// A number or true is not iterable in Python, so it gives nothing here. A
// call site reads a visited item with Get, which gives null on anything but
// an object, as `_dict(item).get(key)` does.
func Elements(value pyjson.Value) []pyjson.Value {
	switch {
	case !value.Truthy():
		return nil
	case value.IsArray():
		return value.Items()
	case value.IsObject():
		keys := value.Keys()
		items := make([]pyjson.Value, len(keys))
		for i, key := range keys {
			items[i] = pyjson.NewString(key)
		}
		return items
	case value.IsString():
		var items []pyjson.Value
		for rest := value.Text(); rest != ""; {
			_, size := pytext.NextCodePoint(rest)
			items = append(items, pyjson.NewString(rest[:size]))
			rest = rest[size:]
		}
		return items
	}
	return nil
}
