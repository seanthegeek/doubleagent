package parserkit

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// Labelled returns label followed by a value's row text, or "" for a value
// jsonvalue.Value.IsEmpty calls empty, so an optional field of a row's
// text is left out whole.
func Labelled(label string, value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.Text()
}
