package parserkit

import "github.com/seanthegeek/doubleagent/internal/jsonvalue"

// Present reports whether a value is neither null (which an absent key
// reads as) nor the empty string, the test the agents' optional fields are
// written under: false, zero, an empty array and an empty object are
// values the agent wrote, and are present.
func Present(value jsonvalue.Value) bool {
	return !value.IsNull() && !IsText(value, "")
}
