package relocation

import (
	"bytes"
	"io"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/glob"
)

// LineName is a relocation variable named on a line of a file.
type LineName struct {
	Line int
	Name string
}

// BoundedNames returns, for each line of r and each of names in order, the
// names the line holds as a whole word: the characters around it, when any,
// are not ASCII letters, digits or underscores. A name matches in its own
// case, as shells name variables; in a PowerShell file it matches in any
// ASCII case, and not before a colon, where it is a drive or scope qualifier
// (`$env:`). A NUL byte reads as a space. The matched text is never returned.
func BoundedNames(r io.Reader, names []string, powerShell bool) ([]LineName, error) {
	words := names
	if powerShell {
		words = make([]string, len(names))
		for i, name := range names {
			words[i] = lowerASCII(name)
		}
	}
	var hits []LineName
	err := eachLine(r, func(number int, line []byte) {
		text := string(bytes.ReplaceAll(line, []byte{0}, []byte{' '}))
		if powerShell {
			text = lowerASCII(text)
		}
		for i, word := range words {
			if bounded(text, word, powerShell) {
				hits = append(hits, LineName{Line: number, Name: names[i]})
			}
		}
	})
	return hits, err
}

// bounded reports whether text holds needle with no word character on
// either side, trying every occurrence, overlapping ones included; in a
// PowerShell file an occurrence followed by a colon does not count.
func bounded(text, needle string, powerShell bool) bool {
	for start := 0; ; {
		offset := strings.Index(text[start:], needle)
		if offset < 0 {
			return false
		}
		position := start + offset
		end := position + len(needle)
		start = position + 1
		if powerShell && end < len(text) && text[end] == ':' {
			continue
		}
		if (position == 0 || !isWordByte(text[position-1])) && (end == len(text) || !isWordByte(text[end])) {
			return true
		}
	}
}

// lowerASCII lowers the ASCII letters of text and leaves every other byte, as
// awk's tolower does under LC_ALL=C.
func lowerASCII(text string) string {
	lowered := []byte(text)
	for i, char := range lowered {
		if char >= 'A' && char <= 'Z' {
			lowered[i] = char + ('a' - 'A')
		}
	}
	return string(lowered)
}

// IsPowerShell reports whether path names a PowerShell file: its name ends in
// `.ps1` in any case.
func IsPowerShell(path string) bool {
	return len(path) >= 4 && path[len(path)-4] == '.' && lowerASCII(path[len(path)-3:]) == "ps1"
}

// Kind names the syntax of a startup file whose values can be read without a
// shell: `environment` for `*/etc/environment`, `environment.d` for a `.conf`
// file under an `environment.d` directory, `launchd` for a plist under a
// `LaunchAgents` or `LaunchDaemons` directory; "" for any other file. The
// patterns are sh `case` patterns, whose `*` crosses `/`.
func Kind(path string) string {
	switch {
	case glob.MatchPath("*/etc/environment", path):
		return "environment"
	case glob.MatchPath("*/environment.d/*.conf", path):
		return "environment.d"
	case glob.MatchPath("*/LaunchAgents/*.plist", path),
		glob.MatchPath("*/LaunchDaemons/*.plist", path):
		return "launchd"
	}
	return ""
}
