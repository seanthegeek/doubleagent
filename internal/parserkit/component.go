package parserkit

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// Component returns name and true when it can stand as one component of a
// path: a name taken from the evidence, such as a session id a database
// row gives, before a parser joins it to a directory to look for a file.
// A name input.PlainName refuses, and on every platform one holding a
// backslash, is refused (false), so a name in the evidence can never reach
// another directory; the caller treats a refused name as a file that is
// not there.
func Component(name string) (string, bool) {
	if !input.PlainName(name) || strings.Contains(name, `\`) {
		return "", false
	}
	return name, true
}
