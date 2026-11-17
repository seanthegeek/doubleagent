package pyjson

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// Format is how an encoder writes a value, as the arguments of Python's
// json.dumps say it. The zero Format is separators=(",", ":") with
// ensure_ascii=False.
type Format struct {
	// SortKeys writes object members in code point order of their keys
	// (sort_keys=True) instead of document order.
	SortKeys bool
	// ASCII writes every character outside printable ASCII, U+007F
	// included, as a \u escape, a pair of them above U+FFFF
	// (ensure_ascii=True).
	ASCII bool
	// Spaced writes ", " between items and ": " after keys, json.dumps's
	// default separators.
	Spaced bool
	// Indent, when above zero, puts each item on its own line indented by
	// that many spaces per level, with "," ending items and ": " after
	// keys, as indent=n does; Spaced is then ignored. Empty arrays and
	// objects stay [] and {}.
	Indent int
}

// Compact returns value as json.dumps(value, ensure_ascii=False,
// separators=(",", ":")) writes it, the form of timeline JSONL records.
func Compact(value Value) string { return Format{}.Encode(value) }

// Sorted returns value as json.dumps(value, ensure_ascii=False,
// separators=(",", ":"), sort_keys=True) writes it.
func Sorted(value Value) string { return Format{SortKeys: true}.Encode(value) }

// Spaced returns value as json.dumps(value) writes it with no arguments.
func Spaced(value Value) string { return Format{Spaced: true, ASCII: true}.Encode(value) }

// SpacedUnicode returns value as json.dumps(value, ensure_ascii=False) writes it.
func SpacedUnicode(value Value) string { return Format{Spaced: true}.Encode(value) }

// Indented returns value as json.dumps(value, indent=2) writes it.
func Indented(value Value) string { return Format{ASCII: true, Indent: 2}.Encode(value) }

// Encode returns value written in format f.
func (f Format) Encode(value Value) string { return string(f.AppendValue(nil, value)) }

// AppendValue appends value written in format f to out.
func (f Format) AppendValue(out []byte, value Value) []byte { return f.appendValue(out, value, 0) }

// appendValue appends value, which sits level arrays or objects deep.
func (f Format) appendValue(out []byte, value Value, level int) []byte {
	switch value.kind {
	case Bool:
		if value.flag {
			return append(out, "true"...)
		}
		return append(out, "false"...)
	case Number:
		if value.flag {
			return append(out, floatText(value.float)...)
		}
		return append(out, intText(value.text)...)
	case String:
		return f.AppendString(out, value.text)
	case Array:
		return f.appendArray(out, value.items, level)
	case Object:
		return f.appendObject(out, value.members.list, level)
	}
	return append(out, "null"...)
}

// appendArray appends the items of an array.
func (f Format) appendArray(out []byte, items []Value, level int) []byte {
	if len(items) == 0 {
		return append(out, "[]"...)
	}
	out = append(out, '[')
	for i, item := range items {
		out = f.appendItemStart(out, i, level+1)
		out = f.appendValue(out, item, level+1)
	}
	return append(f.appendNewline(out, level), ']')
}

// appendObject appends the members of an object, sorted when f says so.
func (f Format) appendObject(out []byte, list []Member, level int) []byte {
	if len(list) == 0 {
		return append(out, "{}"...)
	}
	if f.SortKeys {
		list = slices.Clone(list)
		slices.SortStableFunc(list, func(left, right Member) int {
			return strings.Compare(left.Key, right.Key)
		})
	}
	out = append(out, '{')
	for i, member := range list {
		out = f.appendItemStart(out, i, level+1)
		out = f.AppendString(out, member.Key)
		if f.Spaced || f.Indent > 0 {
			out = append(out, ':', ' ')
		} else {
			out = append(out, ':')
		}
		out = f.appendValue(out, member.Value, level+1)
	}
	return append(f.appendNewline(out, level), '}')
}

// appendItemStart appends what comes before item i of a container whose
// items sit at level: the separator after the previous item, then the
// line break and indentation when f indents.
func (f Format) appendItemStart(out []byte, i, level int) []byte {
	if i > 0 {
		out = append(out, ',')
		if f.Spaced && f.Indent <= 0 {
			out = append(out, ' ')
		}
	}
	return f.appendNewline(out, level)
}

// appendNewline appends a line break and the indentation of level when f
// indents, and nothing otherwise.
func (f Format) appendNewline(out []byte, level int) []byte {
	if f.Indent <= 0 {
		return out
	}
	out = append(out, '\n')
	for range level * f.Indent {
		out = append(out, ' ')
	}
	return out
}

// AppendString appends text as a JSON string in format f: `"` and `\`
// escaped, \b \t \n \f \r by name, other characters below U+0020 as
// \u00XX in lower-case hex, and with ASCII set every character from U+007F
// up as \uXXXX. A lone surrogate is written raw, or as its escape with
// ASCII set. A byte that is neither UTF-8 nor a surrogate's form, which
// the decoder never produces, is written as U+FFFD.
func (f Format) AppendString(out []byte, text string) []byte {
	out = append(out, '"')
	for i := 0; i < len(text); {
		char := text[i]
		if char < utf8.RuneSelf {
			out = f.appendASCII(out, char)
			i++
			continue
		}
		decoded, width := pytext.NextCodePoint(text[i:])
		switch {
		case f.ASCII:
			out = appendUnicodeEscape(out, decoded)
		case decoded == utf8.RuneError && width == 1:
			out = utf8.AppendRune(out, utf8.RuneError)
		default:
			out = append(out, text[i:i+width]...)
		}
		i += width
	}
	return append(out, '"')
}

// appendASCII appends one ASCII character of a string, escaped as needed.
func (f Format) appendASCII(out []byte, char byte) []byte {
	switch char {
	case '"', '\\':
		return append(out, '\\', char)
	case '\b':
		return append(out, '\\', 'b')
	case '\t':
		return append(out, '\\', 't')
	case '\n':
		return append(out, '\\', 'n')
	case '\f':
		return append(out, '\\', 'f')
	case '\r':
		return append(out, '\\', 'r')
	}
	if char < 0x20 || (char == 0x7F && f.ASCII) {
		return appendUnicodeEscape(out, rune(char))
	}
	return append(out, char)
}

// appendUnicodeEscape appends char as \uXXXX in lower-case hex, or as the
// escapes of its UTF-16 surrogate pair above U+FFFF.
func appendUnicodeEscape(out []byte, char rune) []byte {
	if char > 0xFFFF {
		char -= 0x10000
		out = appendUnicodeEscape(out, 0xD800+char>>10)
		return appendUnicodeEscape(out, 0xDC00+char&0x3FF)
	}
	const hexDigits = "0123456789abcdef"
	return append(out, '\\', 'u', hexDigits[char>>12&0xF], hexDigits[char>>8&0xF],
		hexDigits[char>>4&0xF], hexDigits[char&0xF])
}
