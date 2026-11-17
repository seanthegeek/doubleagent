package input

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// textOf returns Python's str(value or ""): "" for a false value (null,
// false, zero, an empty string, array or object), else the value as str()
// prints it.
func textOf(value pyjson.Value) string {
	if !value.Truthy() {
		return ""
	}
	return value.Str()
}

// isText reports whether value is the string text, as Python's == between
// a decoded value and a str literal.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}

// splitSlash splits text at every slash, keeping empty parts, as Python's
// str.split("/").
func splitSlash(text string) []string { return strings.Split(text, "/") }

// joinSlash joins parts with slashes.
func joinSlash(parts []string) string { return strings.Join(parts, "/") }

// parseSize returns Python's int(value or 0) for a manifest size, and false
// when int() would raise: for an array or object, a float that is not
// finite, a string that is not an integer literal, or a value beyond the
// int64 range. A float is truncated toward zero and true counts as 1.
func parseSize(value pyjson.Value) (int64, bool) {
	if !value.Truthy() {
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
		return parseIntegerText(pytext.Strip(value.Text()))
	}
	return 0, false
}

// parseIntegerText parses an integer literal as int() accepts one in
// base 10: an optional sign, then ASCII digits that single underscores
// may separate.
func parseIntegerText(text string) (int64, bool) {
	negative := false
	if text != "" && (text[0] == '+' || text[0] == '-') {
		negative = text[0] == '-'
		text = text[1:]
	}
	var number int64
	for i := 0; i < len(text); i++ {
		character := text[i]
		if character == '_' && i > 0 && i+1 < len(text) && text[i-1] != '_' {
			continue
		}
		if character < '0' || character > '9' || number > (1<<63-1-int64(character-'0'))/10 {
			return 0, false
		}
		number = number*10 + int64(character-'0')
	}
	if text == "" {
		return 0, false
	}
	if negative {
		number = -number
	}
	return number, true
}
