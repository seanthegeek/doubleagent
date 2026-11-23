package parserkit

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// ObjectOrEmpty returns value when it is an object, else a new empty
// object, so a field read from a value of the wrong shape reads as absent.
func ObjectOrEmpty(value jsonvalue.Value) jsonvalue.Value {
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}
