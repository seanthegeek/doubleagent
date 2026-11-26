package input

import (
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

// parseSize returns a manifest size, a JSON integer every collector
// writes, and false for any other value or one beyond the int64 range. A
// missing or null size is 0.
func parseSize(value jsonvalue.Value) (int64, bool) {
	if value.IsNull() {
		return 0, true
	}
	if !value.IsInt() {
		return 0, false
	}
	return value.Int()
}
