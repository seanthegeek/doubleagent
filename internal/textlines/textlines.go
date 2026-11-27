package textlines

import (
	"strings"
	"unicode/utf8"
)

// The character sets a trim takes, each ASCII only.
const (
	// ASCIISpace is the whitespace a line-level trim takes once the line's
	// terminator is gone: space, tab, CR, form feed and vertical tab.
	ASCIISpace = " \t\r\f\v"
	// ASCIIWhitespace is ASCIISpace with LF: all of ASCII's whitespace, the
	// padding a value such as a timestamp may carry.
	ASCIIWhitespace = " \t\n\r\f\v"
	// JSONSpace is the whitespace JSON allows between tokens.
	JSONSpace = " \t\n\r"
	// SpaceOrTab is the whitespace of TOML and of YAML.
	SpaceOrTab = " \t"
)

// Split returns the lines of text, split at LF, each without its LF and
// without one CR before it; the last line also loses one CR that ends the
// text, as a CRLF cut short. A terminating LF gives no empty final line, so
// empty text gives no line; a byte-order mark is kept.
func Split(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

// WithoutTerminator returns a line read from a file less its LF or CRLF
// terminator, if it has one; a CR without an LF after it stays.
func WithoutTerminator(line string) string {
	if !strings.HasSuffix(line, "\n") {
		return line
	}
	return strings.TrimSuffix(line[:len(line)-1], "\r")
}

// ValidUTF8 returns data as text with each byte that is not part of valid
// UTF-8 replaced by U+FFFD, one replacement per byte. It is the rule
// encoding/json applies inside a string and a range loop applies to a
// Go string, so a text decoded here and one decoded as JSON show the same
// replacements for the same bytes.
func ValidUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	return string([]rune(string(data)))
}

// LowerASCII returns text with the bytes A to Z lowered and every other byte
// kept, invalid UTF-8 included, so no non-ASCII letter folds onto an ASCII
// one and the length is unchanged. It returns text itself when no byte
// changes.
func LowerASCII(text string) string {
	for i := range len(text) {
		if LowerByte(text[i]) != text[i] {
			lowered := []byte(text)
			for j := i; j < len(lowered); j++ {
				lowered[j] = LowerByte(lowered[j])
			}
			return string(lowered)
		}
	}
	return text
}

// LowerByte returns char with an ASCII capital lowered and every other byte
// kept, the per-byte form of LowerASCII.
func LowerByte(char byte) byte {
	if 'A' <= char && char <= 'Z' {
		return char + ('a' - 'A')
	}
	return char
}

// UpperASCII returns text with the bytes a to z raised and every other byte
// kept, the mirror of LowerASCII. It returns text itself when no byte
// changes.
func UpperASCII(text string) string {
	for i := range len(text) {
		if UpperByte(text[i]) != text[i] {
			raised := []byte(text)
			for j := i; j < len(raised); j++ {
				raised[j] = UpperByte(raised[j])
			}
			return string(raised)
		}
	}
	return text
}

// UpperByte returns char with an ASCII small letter raised and every other
// byte kept, the per-byte form of UpperASCII.
func UpperByte(char byte) byte {
	if 'a' <= char && char <= 'z' {
		return char - ('a' - 'A')
	}
	return char
}

// EqualFoldASCII reports whether first and second are equal under
// LowerASCII, comparing byte by byte without allocating.
func EqualFoldASCII(first, second string) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range len(first) {
		if LowerByte(first[i]) != LowerByte(second[i]) {
			return false
		}
	}
	return true
}

// HasSuffixFoldASCII reports whether text ends in suffix, ignoring the case
// of ASCII letters as EqualFoldASCII does.
func HasSuffixFoldASCII(text, suffix string) bool {
	return len(text) >= len(suffix) && EqualFoldASCII(text[len(text)-len(suffix):], suffix)
}
