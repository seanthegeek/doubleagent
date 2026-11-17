package parser

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// StripJSONComments returns text with its `//` line comments and `/* */`
// block comments removed where they stand outside a string, the form of
// JSON with comments that several agents' settings files use. A line
// comment ends before its LF, which stays; an unclosed block comment runs to
// the end. Strings are copied as they are, a backslash escaping the
// character after it.
func StripJSONComments(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	inString := false
	for i := 0; i < len(text); {
		character := text[i]
		switch {
		case inString:
			if character == '\\' && i+1 < len(text) {
				out.WriteString(text[i : i+2])
				i += 2
				continue
			}
			inString = character != '"'
		case character == '"':
			inString = true
		case strings.HasPrefix(text[i:], "//"):
			i = indexFrom(text, i, "\n", 0)
			continue
		case strings.HasPrefix(text[i:], "/*"):
			i = indexFrom(text, i+2, "*/", 2)
			continue
		}
		out.WriteByte(character)
		i++
	}
	return out.String()
}

// indexFrom returns the offset of the first separator in text at or after
// start, plus skip, or the length of text when there is none.
func indexFrom(text string, start int, separator string, skip int) int {
	found := strings.Index(text[start:], separator)
	if found < 0 {
		return len(text)
	}
	return start + found + skip
}

// StripTrailingCommas returns text without each comma outside a string that
// only JSON whitespace separates from a closing `}` or `]`: the trailing
// commas a lenient JSON-with-comments reader accepts. Run it after
// StripJSONComments.
func StripTrailingCommas(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	inString := false
	for i := 0; i < len(text); i++ {
		character := text[i]
		switch {
		case inString:
			if character == '\\' && i+1 < len(text) {
				out.WriteString(text[i : i+2])
				i++
				continue
			}
			inString = character != '"'
		case character == '"':
			inString = true
		case character == ',':
			next := skipJSONWhitespace(text, i+1)
			if next < len(text) && (text[next] == '}' || text[next] == ']') {
				continue
			}
		}
		out.WriteByte(character)
	}
	return out.String()
}

// StripJSONC returns JSON with comments and trailing commas as plain JSON:
// StripTrailingCommas over StripJSONComments. A byte-order mark is kept, so
// a reader that refuses one still does.
func StripJSONC(text string) string {
	return StripTrailingCommas(StripJSONComments(text))
}

// ParseJSONC decodes a JSON-with-comments document: raw bytes decoded as
// UTF-8 with replacement, a leading byte-order mark dropped, comments and
// trailing commas stripped. It returns the decoder's error for text that is
// still not JSON.
func ParseJSONC(raw []byte) (pyjson.Value, error) {
	text := strings.TrimPrefix(pytext.DecodeUTF8(raw), byteOrderMark)
	return pyjson.DecodeString(StripJSONC(text))
}
