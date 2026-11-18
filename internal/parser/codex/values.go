package codex

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// textIn reports whether value is a string among texts.
func textIn(value jsonvalue.Value, texts ...string) bool {
	return value.IsString() && slices.Contains(texts, value.StringValue())
}

// isMissing reports whether value is null or the empty string: false and
// zero are values the agent wrote.
func isMissing(value jsonvalue.Value) bool {
	return value.IsNull() || isText(value, "")
}

// isNullOrZero reports whether value is null, false or a number equal to
// zero: an exit code that reports no failure.
func isNullOrZero(value jsonvalue.Value) bool {
	return value.IsEmpty() && (value.IsNull() || value.IsBool() || value.IsNumber())
}

// sameValue reports whether two decoded values hold the same value:
// numbers by value, with a boolean as the number 0 or 1 and two integers
// by their text, strings by text, null only to null, and arrays and objects
// as jsonvalue.Equal compares them.
func sameValue(left, right jsonvalue.Value) bool {
	leftNumber, leftIsNumber := numeric(left)
	rightNumber, rightIsNumber := numeric(right)
	switch {
	case leftIsNumber || rightIsNumber:
		if left.IsInt() && right.IsInt() {
			return left.Number() == right.Number()
		}
		return leftIsNumber && rightIsNumber && leftNumber == rightNumber
	case left.Kind() != right.Kind():
		return false
	case left.IsString():
		return left.StringValue() == right.StringValue()
	}
	return jsonvalue.Equal(left, right)
}

// numeric returns a number as a float, and a boolean as 0 or 1.
func numeric(value jsonvalue.Value) (float64, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return 1, true
		}
		return 0, true
	}
	if !value.IsNumber() {
		return 0, false
	}
	number, _ := value.Float()
	return number, true
}

// without returns an object's members other than the named keys, as a new
// object in the same order; a value that is not an object gives an empty
// object.
func without(object jsonvalue.Value, keys ...string) jsonvalue.Value {
	var kept []jsonvalue.Member
	for _, member := range object.Members() {
		if !slices.Contains(keys, member.Key) {
			kept = append(kept, member)
		}
	}
	return jsonvalue.NewObject(kept...)
}

// objectOr returns value when it is an object, else an empty object, so a
// field of the wrong type reads as one with no members.
func objectOr(value jsonvalue.Value) jsonvalue.Value {
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}
