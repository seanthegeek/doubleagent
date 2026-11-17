package input

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// textOf returns a manifest field read as text: a string as it is, an
// empty value (null, false, zero, an empty string, array or object) as "",
// and any other value as its row text (jsonvalue.Value.Text), the fallback
// for a writer that put a number or a boolean where text belongs.
func textOf(value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.Text()
}

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// splitSlash splits text at every slash, keeping empty parts.
func splitSlash(text string) []string { return strings.Split(text, "/") }

// joinSlash joins parts with slashes.
func joinSlash(parts []string) string { return strings.Join(parts, "/") }

// parseSize returns a manifest size read as an integer, and false when it
// is not one. An empty value is 0. The fallbacks for a writer that did not
// write an integer: true is 1, a finite float is cut toward zero, and a
// string is read as a base-10 integer with an optional sign, surrounding
// whitespace ignored. An array, an object, other text and a value beyond
// the int64 range are refused.
func parseSize(value jsonvalue.Value) (int64, bool) {
	if value.IsEmpty() {
		return 0, true
	}
	switch {
	case value.IsBool():
		return 1, true
	case value.IsInt():
		return value.Int()
	case value.IsFloat():
		number, _ := value.Float()
		if number != number || number >= 9.223372036854775807e18 || number < -9.223372036854775808e18 {
			return 0, false
		}
		return int64(number), true
	case value.IsString():
		number, err := strconv.ParseInt(strings.TrimSpace(value.StringValue()), 10, 64)
		return number, err == nil
	}
	return 0, false
}
