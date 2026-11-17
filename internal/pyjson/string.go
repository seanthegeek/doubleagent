package pyjson

import (
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// stringBody reads the string whose opening quote is at offset quote and
// returns its text and the offset after the closing quote. A string of
// plain characters is returned as a slice of the document; one with an
// escape or an invalid byte is rebuilt.
func (d *decoder) stringBody(quote int) (string, int, error) {
	text := d.text
	for i := quote + 1; i < len(text); {
		char := text[i]
		switch {
		case char == '"':
			return text[quote+1 : i], i + 1, nil
		case char == '\\' || char < 0x20:
			return d.rebuildString(quote, i)
		case char < utf8.RuneSelf:
			i++
		default:
			decoded, width := pytext.NextCodePoint(text[i:])
			if decoded == utf8.RuneError && width == 1 {
				return d.rebuildString(quote, i)
			}
			i += width
		}
	}
	return "", 0, newSyntaxError(text, quote, "unterminated string")
}

// rebuildString continues reading the string whose quote is at offset
// quote from offset i, where an escape, a control character or an invalid
// byte is, decoding escapes and replacing invalid bytes with U+FFFD. A
// control character below U+0020 is an error, as in Python's strict mode.
func (d *decoder) rebuildString(quote, i int) (string, int, error) {
	text := d.text
	out := []byte(text[quote+1 : i])
	for i < len(text) {
		char := text[i]
		switch {
		case char == '"':
			return string(out), i + 1, nil
		case char < 0x20:
			return "", 0, newSyntaxError(text, i, "invalid control character in string")
		case char == '\\':
			var err error
			if out, i, err = d.escape(out, quote, i); err != nil {
				return "", 0, err
			}
		case char < utf8.RuneSelf:
			out = append(out, char)
			i++
		default:
			decoded, width := pytext.NextCodePoint(text[i:])
			out = pytext.AppendCodePoint(out, decoded)
			i += width
		}
	}
	return "", 0, newSyntaxError(text, quote, "unterminated string")
}

// escape appends the character the escape at offset i stands for to out
// and returns the offset after it. A \u escape of a high surrogate followed
// by one of a low surrogate gives the one character they encode; any other
// surrogate stays a lone surrogate.
func (d *decoder) escape(out []byte, quote, i int) ([]byte, int, error) {
	text := d.text
	if i+1 >= len(text) {
		return nil, 0, newSyntaxError(text, quote, "unterminated string")
	}
	if replacement, ok := shortEscape(text[i+1]); ok {
		return append(out, replacement), i + 2, nil
	}
	if text[i+1] != 'u' {
		return nil, 0, newSyntaxError(text, i, "invalid escape")
	}
	char, ok := hexQuad(text, i+2)
	if !ok {
		return nil, 0, newSyntaxError(text, i, "invalid \\u escape")
	}
	end := i + 6
	if char >= 0xD800 && char <= 0xDBFF && end+1 < len(text) && text[end] == '\\' &&
		text[end+1] == 'u' {
		if low, ok := hexQuad(text, end+2); ok && low >= 0xDC00 && low <= 0xDFFF {
			char = 0x10000 + (char-0xD800)<<10 + (low - 0xDC00)
			end += 6
		}
	}
	return pytext.AppendCodePoint(out, char), end, nil
}

// shortEscape returns the character a one-letter escape after a backslash
// stands for, and whether letter is one.
func shortEscape(letter byte) (byte, bool) {
	switch letter {
	case '"', '\\', '/':
		return letter, true
	case 'b':
		return '\b', true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	}
	return 0, false
}

// hexQuad reads the four hexadecimal digits at offset i, in either case.
func hexQuad(text string, i int) (rune, bool) {
	if i+4 > len(text) {
		return 0, false
	}
	var value rune
	for _, digit := range []byte(text[i : i+4]) {
		switch {
		case digit >= '0' && digit <= '9':
			value = value<<4 | rune(digit-'0')
		case digit >= 'a' && digit <= 'f':
			value = value<<4 | rune(digit-'a'+10)
		case digit >= 'A' && digit <= 'F':
			value = value<<4 | rune(digit-'A'+10)
		default:
			return 0, false
		}
	}
	return value, true
}
