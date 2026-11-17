package parser

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// TextOf returns the text of a chat message's content: a string as it is;
// for a list of blocks, the strings and each block's string "text" joined
// by LF, an image block without text as "[image]"; for an object, its
// string "text", else the object as JSON (", " and ": " separators,
// non-ASCII kept); "" for null; any other value as Python's str() writes it.
func TextOf(content pyjson.Value) string {
	switch {
	case content.IsNull():
		return ""
	case content.IsString():
		return content.Text()
	case content.IsArray():
		var parts []string
		for _, block := range content.Items() {
			switch {
			case block.IsString():
				parts = append(parts, block.Text())
			case !block.IsObject():
			case block.Get("text").IsString():
				parts = append(parts, block.Get("text").Text())
			case isString(block.Get("type"), "image"):
				parts = append(parts, "[image]")
			}
		}
		return strings.Join(parts, "\n")
	case content.IsObject():
		if text := content.Get("text"); text.IsString() {
			return text.Text()
		}
		return pyjson.SpacedUnicode(content)
	}
	return content.Str()
}

// Str returns a value as Python's str(value or "") gives it, the form most
// parsers read an identifier or a name in: "" for a value Python counts as
// false (null, false, zero, an empty string, array or object), else the
// value's str().
func Str(value pyjson.Value) string {
	if !value.Truthy() {
		return ""
	}
	return value.Str()
}

// isString reports whether value is the string text.
func isString(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}

// CompactJSON returns a value as compact JSON with sorted keys and non-ASCII
// text kept, the form parsers give a structured value inside row text.
func CompactJSON(value pyjson.Value) string {
	return pyjson.Sorted(value)
}

// SettingValue returns a setting as row text: "true" or "false" for a
// boolean, a string as it is, anything else as CompactJSON.
func SettingValue(value pyjson.Value) string {
	if flag, ok := value.Bool(); ok {
		if flag {
			return "true"
		}
		return "false"
	}
	if value.IsString() {
		return value.Text()
	}
	return CompactJSON(value)
}

// SettingOrDash returns a record value as row text, "-" when it is null or
// the empty string: SettingValue of anything else, so false is "false" and
// zero is "0". It is the form parsers give a field of a summary row that may
// be absent.
func SettingOrDash(value pyjson.Value) string {
	if value.IsNull() || isString(value, "") {
		return "-"
	}
	return SettingValue(value)
}

// Tracker remembers, per key (a session or thread id), the last value of
// one tracked state such as the model, for one parse.
type Tracker map[string]string

// Change returns the system row text for a change of the tracked state of
// key: "<label>: <value>" the first time key has a value and
// "<label>: <old> -> <new>" on a change. An empty or unchanged value gives
// "" and is not remembered.
func (t Tracker) Change(label, key, value string) string {
	if value == "" {
		return ""
	}
	old, seen := t[key]
	if seen && old == value {
		return ""
	}
	t[key] = value
	if old != "" {
		return label + ": " + old + " -> " + value
	}
	return label + ": " + value
}
