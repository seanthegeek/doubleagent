package parserkit

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// Elements returns the items of an array, and nothing for any other value.
// A call site reads a field of an item with Get, which gives null on
// anything but an object.
func Elements(value jsonvalue.Value) []jsonvalue.Value {
	return value.Items()
}
