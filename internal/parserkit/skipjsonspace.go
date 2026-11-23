package parserkit

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// SkipJSONSpace returns the first offset at or after offset that is not
// the whitespace JSON allows between tokens (textlines.JSONSpace: space,
// tab, line feed and carriage return), or the length of text.
func SkipJSONSpace(text string, offset int) int {
	for offset < len(text) && strings.IndexByte(textlines.JSONSpace, text[offset]) >= 0 {
		offset++
	}
	return offset
}
