package codex

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// strOr returns a value as Python's str() writes it when Python counts it
// as true, else fallback: the form of `value or fallback` written with %s.
func strOr(value jsonvalue.Value, fallback string) string {
	if !value.IsEmpty() {
		return value.Text()
	}
	return fallback
}

// orDash returns text, or "-" when it is empty.
func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}

// firstTruthy returns the first value Python counts as true, else the last
// value, as a chain of `or` does.
func firstTruthy(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values {
		if !value.IsEmpty() {
			return value
		}
	}
	if len(values) == 0 {
		return jsonvalue.NewNull()
	}
	return values[len(values)-1]
}

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// textIn reports whether value is a string among texts.
func textIn(value jsonvalue.Value, texts ...string) bool {
	return value.IsString() && slices.Contains(texts, value.StringValue())
}

// isNoneOrEmpty reports whether value is null or the empty string, the
// test `value in (None, "")`.
func isNoneOrEmpty(value jsonvalue.Value) bool {
	return value.IsNull() || isText(value, "")
}

// isNoneOrZero reports whether value is null, false or a number equal to
// zero, the test `value in (None, 0)`.
func isNoneOrZero(value jsonvalue.Value) bool {
	return value.IsEmpty() && (value.IsNull() || value.IsBool() || value.IsNumber())
}

// sameValue reports whether two decoded values are equal as Python compares
// them: numbers and booleans by numeric value, strings by text, null only to
// null, and lists and dicts by their JSON form.
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

// numeric returns a number or boolean as a float, Python's True being 1.
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

// objectOr returns value when it is an object, else an empty object, the
// form `value if isinstance(value, dict) else {}`.
func objectOr(value jsonvalue.Value) jsonvalue.Value {
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}
