package parser

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// CopyObject returns a new object holding an object's members in their
// order, so setting a key on it leaves the original as it was; a value that
// is not an object gives an empty object.
func CopyObject(object jsonvalue.Value) jsonvalue.Value {
	return jsonvalue.NewObject(object.Members()...)
}
