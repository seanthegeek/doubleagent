package parser

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// TextOf returns the text of a chat message's content: a string as it is;
// for a list of blocks, the strings and each block's string "text" joined
// by LF, an image block without text as "[image]"; for an object, its
// string "text", else the object as JSONText; "" for null; any other value
// as its row text (jsonvalue.Value.Text).
func TextOf(content jsonvalue.Value) string {
	switch {
	case content.IsNull():
		return ""
	case content.IsString():
		return content.StringValue()
	case content.IsArray():
		var parts []string
		for _, block := range content.Items() {
			switch {
			case block.IsString():
				parts = append(parts, block.StringValue())
			case !block.IsObject():
			case block.Get("text").IsString():
				parts = append(parts, block.Get("text").StringValue())
			case IsText(block.Get("type"), "image"):
				parts = append(parts, "[image]")
			}
		}
		return strings.Join(parts, "\n")
	case content.IsObject():
		if text := content.Get("text"); text.IsString() {
			return text.StringValue()
		}
		return JSONText(content)
	}
	return content.Text()
}

// Text returns a value as row text, the form most parsers read an
// identifier or a name in: "" for an empty value (null, false, zero, an
// empty string, array or object), else jsonvalue.Value.Text: a string as it
// is, a number as written, true, or an array or object as JSONText.
func Text(value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.Text()
}

// JSONText returns a value as the JSON a row embeds: compact, keys in the
// evidence's order, numbers as written, non-ASCII text kept, and U+2028
// and U+2029 escaped as encoding/json escapes them.
func JSONText(value jsonvalue.Value) string {
	return value.JSON()
}

// SettingValue returns a value as row text the way a setting, or any value
// a row shows whole, is written: a string as it is, anything else as
// JSONText, so a boolean is "true" or "false".
func SettingValue(value jsonvalue.Value) string {
	if flag, ok := value.Bool(); ok {
		if flag {
			return "true"
		}
		return "false"
	}
	if value.IsString() {
		return value.StringValue()
	}
	return JSONText(value)
}

// SettingOrDash returns a record value as row text, "-" when it is null or
// the empty string: SettingValue of anything else, so false is "false" and
// zero is "0". It is the form parsers give a field of a summary row that may
// be absent.
func SettingOrDash(value jsonvalue.Value) string {
	if !Present(value) {
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
