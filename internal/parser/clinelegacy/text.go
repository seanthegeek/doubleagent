package clinelegacy

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// Present reports whether a value is neither null nor the empty string, the
// test the agents' optional fields are written under: false, zero, an
// empty array and an empty object are present.
func Present(value pyjson.Value) bool {
	return !value.IsNull() && !(value.IsString() && value.Text() == "")
}

// FormatCounts returns "key=value" for each key whose value in record is
// Present, joined by spaces, each value as Python's str() writes it.
func FormatCounts(record pyjson.Value, keys ...string) string {
	var parts []string
	for _, key := range keys {
		if value := record.Get(key); Present(value) {
			parts = append(parts, key+"="+value.Str())
		}
	}
	return strings.Join(parts, " ")
}

// Join returns the parts that are not null, an empty string, an empty array
// or an empty object, each as Python's str() writes it, joined by " | ".
func Join(parts ...pyjson.Value) string {
	var kept []string
	for _, part := range parts {
		empty := (part.IsArray() || part.IsObject()) && part.Len() == 0
		if Present(part) && !empty {
			kept = append(kept, part.Str())
		}
	}
	return strings.Join(kept, " | ")
}

// FirstTruthy returns the first value Python counts as true, else the last one, as
// Python's `a or b or c` does.
func FirstTruthy(values ...pyjson.Value) pyjson.Value {
	for _, value := range values[:len(values)-1] {
		if value.Truthy() {
			return value
		}
	}
	return values[len(values)-1]
}

// LoadsOrNone returns text decoded as a JSON object, or false when it does
// not open with "{" after leading whitespace or is not a JSON object.
func LoadsOrNone(text string) (pyjson.Value, bool) {
	if !strings.HasPrefix(pytext.StripLeft(text), "{") {
		return pyjson.NewNull(), false
	}
	value, err := pyjson.DecodeString(text)
	if err != nil || !value.IsObject() {
		return pyjson.NewNull(), false
	}
	return value, true
}

// IsNumber reports whether a value is a number for Python's
// isinstance(value, (int, float)), which a boolean is too.
func IsNumber(value pyjson.Value) bool {
	return value.IsNumber() || value.IsBool()
}

// NumberOf returns a number as a float, a boolean as 0 or 1, and false for
// any other value or an integer too large for a float.
func NumberOf(value pyjson.Value) (float64, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return 1, true
		}
		return 0, true
	}
	return value.Float()
}

// ArrayAfterKey returns the byte offset of the "[" that opens the array of
// the first `"<key>"` in text followed, after JSON whitespace only, by a
// colon and the bracket, so a file cut mid-write can be walked from there.
// It stands for the Python search `"<key>"[ \t\n\r]*:[ \t\n\r]*(?=\[)`,
// whose lookahead Go's engine has no form for.
func ArrayAfterKey(text, key string) (int, bool) {
	quoted := `"` + key + `"`
	for start := 0; ; start++ {
		found := strings.Index(text[start:], quoted)
		if found < 0 {
			return 0, false
		}
		start += found
		cursor := skipWhitespace(text, start+len(quoted))
		if cursor < len(text) && text[cursor] == ':' {
			cursor = skipWhitespace(text, cursor+1)
			if cursor < len(text) && text[cursor] == '[' {
				return cursor, true
			}
		}
	}
}

// skipWhitespace returns the first offset at or after offset that is not
// JSON whitespace.
func skipWhitespace(text string, offset int) int {
	for offset < len(text) && strings.IndexByte(" \t\r\n", text[offset]) >= 0 {
		offset++
	}
	return offset
}

// CompareNumbers returns -1, 0 or 1 as left is below, equal to or above
// right, the order a sort by number gives.
func CompareNumbers(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}
