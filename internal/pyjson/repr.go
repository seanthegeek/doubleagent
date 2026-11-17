package pyjson

import (
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// Str returns v as Python's str(v) writes the decoded value, the text "%s"
// gives: a string as it is, an integer's digits, a float as repr writes it
// (1.0, 1e+16, nan, inf), True, False, None, and an array or object as
// Python writes a list or dict (Repr).
func (v Value) Str() string {
	if v.kind == String {
		return v.text
	}
	return v.Repr()
}

// Repr returns v as Python's repr(v) writes the decoded value: strings
// quoted as repr quotes them, lists as [a, b] and dicts as {'k': v}.
func (v Value) Repr() string {
	var out strings.Builder
	v.appendRepr(&out)
	return out.String()
}

// appendRepr writes repr(v) to out.
func (v Value) appendRepr(out *strings.Builder) {
	switch v.kind {
	case Null:
		out.WriteString(pytext.None)
	case Bool:
		out.WriteString(pytext.Bool(v.flag))
	case Number:
		if v.flag {
			out.WriteString(pytext.Float(v.float))
		} else {
			out.WriteString(intText(v.text))
		}
	case String:
		out.WriteString(stringRepr(v.text))
	case Array:
		out.WriteByte('[')
		for i, item := range v.items {
			if i > 0 {
				out.WriteString(", ")
			}
			item.appendRepr(out)
		}
		out.WriteByte(']')
	case Object:
		out.WriteByte('{')
		for i, member := range v.members.list {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(stringRepr(member.Key))
			out.WriteString(": ")
			member.Value.appendRepr(out)
		}
		out.WriteByte('}')
	}
}

// stringRepr returns repr(text) as Python writes a str: in single quotes,
// or double quotes when the text holds a single quote and no double quote;
// the quote and backslash escaped, \t \n \r by name, other control
// characters and U+007F as \xNN, and characters Python does not count as
// printable (separators, format and unassigned characters, surrogates) as
// \xNN, \uNNNN or \UNNNNNNNN.
func stringRepr(text string) string {
	quote := byte('\'')
	if strings.IndexByte(text, '\'') >= 0 && strings.IndexByte(text, '"') < 0 {
		quote = '"'
	}
	out := []byte{quote}
	for i := 0; i < len(text); {
		char, width := pytext.NextCodePoint(text[i:])
		switch {
		case char == rune(quote) || char == '\\':
			out = append(out, '\\', byte(char))
		case char == '\t':
			out = append(out, '\\', 't')
		case char == '\n':
			out = append(out, '\\', 'n')
		case char == '\r':
			out = append(out, '\\', 'r')
		case char < 0x7F && char >= 0x20:
			out = append(out, byte(char))
		case char < 0x7F || char == 0x7F || !isPrintable(char):
			out = appendReprEscape(out, char)
		default:
			out = utf8.AppendRune(out, char)
		}
		i += width
	}
	return string(append(out, quote))
}

// appendReprEscape appends char as repr escapes it: \xNN up to U+00FF,
// \uNNNN up to U+FFFF and \UNNNNNNNN above, in lower-case hex.
func appendReprEscape(out []byte, char rune) []byte {
	const hexDigits = "0123456789abcdef"
	digits := 8
	switch {
	case char <= 0xFF:
		out, digits = append(out, '\\', 'x'), 2
	case char <= 0xFFFF:
		out, digits = append(out, '\\', 'u'), 4
	default:
		out = append(out, '\\', 'U')
	}
	for shift := (digits - 1) * 4; shift >= 0; shift -= 4 {
		out = append(out, hexDigits[char>>uint(shift)&0xF])
	}
	return out
}
