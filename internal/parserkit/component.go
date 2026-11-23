package parserkit

import "strings"

// Component returns name and true when it can stand as one component of a
// path: a name taken from the evidence, such as a session id a database
// row gives, before a parser joins it to a directory to look for a file.
// An empty name, "." and "..", and any name holding a slash, a backslash or
// a NUL byte are refused (false), so a name in the evidence can never
// reach another directory; the caller treats a refused name as a file that
// is not there.
func Component(name string) (string, bool) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return "", false
	}
	return name, true
}
