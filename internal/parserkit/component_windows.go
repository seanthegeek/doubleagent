package parserkit

import "strings"

// platformName reports whether this platform holds name as one component
// below a directory: on Windows a colon names a drive or an alternate data
// stream of a file, so a name holding one is refused, as Under refuses it.
func platformName(name string) bool { return !strings.Contains(name, ":") }
