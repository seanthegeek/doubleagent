package clinelegacy

import (
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// Present reports whether a value is neither null nor the empty string, the
// test the agents' optional fields are written under: false, zero, an
// empty array and an empty object are present.
func Present(value jsonvalue.Value) bool {
	return !value.IsNull() && !(value.IsString() && value.StringValue() == "")
}

// FormatCounts returns "key=value" for each key whose value in record is
// Present, joined by spaces, each value as its row text
// (jsonvalue.Value.Text).
func FormatCounts(record jsonvalue.Value, keys ...string) string {
	var parts []string
	for _, key := range keys {
		if value := record.Get(key); Present(value) {
			parts = append(parts, key+"="+value.Text())
		}
	}
	return strings.Join(parts, " ")
}

// Join returns the parts that are not null, an empty string, an empty array
// or an empty object, each as its row text, joined by " | ". False and zero
// are kept: they are values the agent wrote.
func Join(parts ...jsonvalue.Value) string {
	var kept []string
	for _, part := range parts {
		empty := (part.IsArray() || part.IsObject()) && part.Len() == 0
		if Present(part) && !empty {
			kept = append(kept, part.Text())
		}
	}
	return strings.Join(kept, " | ")
}

// LoadsOrNone returns text decoded as a JSON object, or false when it does
// not open with "{" after leading whitespace or is not a JSON object.
func LoadsOrNone(text string) (jsonvalue.Value, bool) {
	if !strings.HasPrefix(strings.TrimLeftFunc(text, unicode.IsSpace), "{") {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.DecodeString(text)
	if err != nil || !value.IsObject() {
		return jsonvalue.NewNull(), false
	}
	return value, true
}

// IsNumber reports whether a value is read as a number: a JSON number, or
// a boolean, which counts as 0 or 1 (NumberOf) wherever a time or a count
// is expected.
func IsNumber(value jsonvalue.Value) bool {
	return value.IsNumber() || value.IsBool()
}

// NumberOf returns a number as a float, a boolean as 0 or 1, and false for
// any other value or an integer too large for a float.
func NumberOf(value jsonvalue.Value) (float64, bool) {
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
// As a pattern it would be `"<key>"[ \t\n\r]*:[ \t\n\r]*(?=\[)`, whose
// lookahead Go's regexp has no form for.
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
