package analyze

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// shown returns text as the analysis commands print it for a person, on a
// terminal: each C0 control character (a line end and a tab included), DEL
// and each byte that is not valid UTF-8 as \xNN, and each C1 control
// (U+0080 to U+009F) as \u00NN, everything else as written. Text from the
// evidence, a host name, a user directory's name or a note, can then
// neither move the cursor, clear the screen or retitle the window, nor
// break a line or a table; a writer adds its own line ends and separators
// after it. The output files carry the text as written, escaped by their
// own formats.
func shown(text string) string {
	if !needsShowing(text) {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); {
		char, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case char == utf8.RuneError && size == 1, char < 0x20, char == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, text[i])
		case char >= 0x80 && char <= 0x9f:
			fmt.Fprintf(&out, `\u%04x`, char)
		default:
			out.WriteString(text[i : i+size])
		}
		i += size
	}
	return out.String()
}

// needsShowing reports whether text holds a byte shown changes: below
// 0x20, DEL, or one at or above 0x80, which may begin a C1 control or a
// byte that is not UTF-8.
func needsShowing(text string) bool {
	for i := range len(text) {
		if text[i] < 0x20 || text[i] >= 0x7f {
			return true
		}
	}
	return false
}
