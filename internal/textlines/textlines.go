// Package textlines splits text into lines and names the character sets
// the analyzer trims, as the formats it reads specify them.
//
// A reader of a configuration or log format ends a line at LF or CRLF
// only and trims only ASCII whitespace. Unicode-aware splitting would also
// end a line at U+2028, U+0085 or a form feed, and a Unicode trim would
// take U+00A0 and other Unicode spaces, so either would cut or shorten a
// value that holds one. A lone CR stays inside its line. The sets are
// meant for strings.Trim, strings.TrimLeft and strings.TrimRight, which
// read them as single bytes here since every character is ASCII.
package textlines

import "strings"

// The character sets a trim takes, each ASCII only.
const (
	// ASCIISpace is the whitespace a line-level trim takes once the line's
	// terminator is gone: space, tab, CR, form feed and vertical tab.
	ASCIISpace = " \t\r\f\v"
	// ASCIIWhitespace is ASCIISpace with LF: Python's ASCII whitespace, the
	// padding a value such as a timestamp may carry.
	ASCIIWhitespace = " \t\n\r\f\v"
	// JSONSpace is the whitespace JSON allows between tokens.
	JSONSpace = " \t\n\r"
	// SpaceOrTab is the whitespace of TOML and of YAML.
	SpaceOrTab = " \t"
)

// Split returns the lines of text, split at LF, each without its LF and
// without one CR before it. A terminating LF gives no empty final line, so
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
