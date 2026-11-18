package clinelegacy

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// FormatCounts returns "key=value" for each key whose value in record is
// parser.Present, joined by spaces, each value as its row text
// (jsonvalue.Value.Text).
func FormatCounts(record jsonvalue.Value, keys ...string) string {
	var parts []string
	for _, key := range keys {
		if value := record.Get(key); parser.Present(value) {
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
		if parser.Present(part) && !empty {
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
// any other value or a number beyond the float64 range.
func NumberOf(value jsonvalue.Value) (float64, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return 1, true
		}
		return 0, true
	}
	return value.Float()
}

// OrderOf returns the number a time sorts by: NumberOf's number, else as
// parser.NumberOrder orders it, a number beyond the float64 range past
// every other.
func OrderOf(value jsonvalue.Value) float64 {
	if number, ok := NumberOf(value); ok {
		return number
	}
	return parser.NumberOrder(value)
}

// TimeOf is the one reading of a time in epoch milliseconds that a task's
// lookups and orders share: a JSON number, a boolean as 0 or 1, and a
// number past the float64 range as +Inf or -Inf by its sign (OrderOf), so
// it falls after or before every real time, never at 0. Any other value is
// not a time: false.
func TimeOf(value jsonvalue.Value) (float64, bool) {
	if !IsNumber(value) {
		return 0, false
	}
	return OrderOf(value), true
}

// KeyedArray returns the pattern of `"<key>"` followed, after JSON
// whitespace only, by a colon and the "[" that opens the key's array. key
// is a constant name the caller holds the pattern of.
func KeyedArray(key string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(`"`+key+`"`) + `[ \t\n\r]*:[ \t\n\r]*\[`)
}

// ArrayAfterKey returns the byte offset of the "[" that ends the first
// match of a KeyedArray pattern in text, so a file cut mid-write can be
// walked from there.
func ArrayAfterKey(text string, keyed *regexp.Regexp) (int, bool) {
	found := keyed.FindStringIndex(text)
	if found == nil {
		return 0, false
	}
	return found[1] - 1, true
}
