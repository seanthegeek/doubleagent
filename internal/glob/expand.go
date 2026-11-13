package glob

import (
	"os"
	"slices"
	"strings"
)

// Expand returns base + "/" + match for every match of pattern inside base,
// an absolute path or empty for the root, never itself a pattern, as the sh
// collector's `cd -P BASE` and unquoted `for` loop give them: the pattern is
// split into words at newlines, as with IFS set to newline only; each word's
// matches are sorted bytewise; a word that matches nothing is kept as typed,
// escapes and all, when an entry by that name exists, and dropped otherwise.
// Directories are read as the shell reads them, so a symlinked directory in
// the middle of the pattern is followed.
func Expand(base, pattern string) []string {
	var results []string
	for _, word := range strings.Split(pattern, "\n") {
		if word == "" {
			continue
		}
		var matches []string
		if strings.ContainsAny(word, "*?[") {
			expandFrom(base, "", word, &matches)
		}
		if len(matches) == 0 {
			if _, err := os.Lstat(base + "/" + word); err == nil {
				results = append(results, base+"/"+word)
			}
			continue
		}
		slices.Sort(matches)
		for _, match := range matches {
			results = append(results, base+"/"+match)
		}
	}
	return results
}

// expandFrom appends to matches each path, relative to base, that extends
// prefix (empty or ending in `/`) by a match of rest. Literal segments
// before the first wildcard segment are appended unread; the wildcard
// segment's directory is read; a literal tail after the last wildcard
// segment is kept only when os.Lstat finds it. A whole pattern with no
// wildcard segment gives nothing here: Expand falls back to the word itself.
func expandFrom(base, prefix, rest string, matches *[]string) {
	segmentAt, segmentEnd, separatorWidth, found := findWildcardSegment(rest)
	if !found {
		if prefix == "" {
			return
		}
		path := prefix + unescape(rest)
		if _, err := os.Lstat(base + "/" + path); err == nil {
			*matches = append(*matches, path)
		}
		return
	}
	prefix += unescape(rest[:segmentAt])
	segment, atEnd := rest[segmentAt:segmentEnd], segmentEnd == len(rest)
	for _, name := range readNames(base+"/"+prefix, segmentMatchesDot(segment)) {
		if (strings.HasPrefix(name, ".") && !segmentMatchesDot(segment)) ||
			!match(segment, name) {
			continue
		}
		if atEnd {
			*matches = append(*matches, prefix+name)
		} else {
			expandFrom(base, prefix+name+"/", rest[segmentEnd+separatorWidth:], matches)
		}
	}
}

// readNames returns the entry names of directory, with `.` and `..` added
// when withDots is set, since the shell's directory read returns them and a
// segment starting with a dot matches them. A directory that cannot be read
// gives the names read before the error, as the shell's read loop stops.
func readNames(directory string, withDots bool) []string {
	handle, err := os.Open(directory)
	if err != nil {
		return nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	names := make([]string, 0, len(entries)+2)
	if withDots {
		names = append(names, ".", "..")
	}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// findWildcardSegment finds the first segment of rest holding a wildcard and
// returns where it starts and ends and the width of the separator after it:
// 1 for `/`, 2 for `\/`, which the shell also takes as a separator. A `[`
// counts as a wildcard only when a `]` closes it within the segment, after
// at least one byte (and after the `!`), as the shell's scan decides.
func findWildcardSegment(rest string) (segmentAt, segmentEnd, separatorWidth int, found bool) {
	for at := 0; at < len(rest); {
		switch rest[at] {
		case '*', '?':
			found = true
		case '[':
			found = found || bracketClosesInSegment(rest, at+1)
		default:
			escape := 0
			if rest[at] == '\\' && at+1 < len(rest) {
				escape = 1
			}
			if rest[at+escape] == '/' {
				if found {
					return segmentAt, at, escape + 1, true
				}
				segmentAt = at + escape + 1
			}
			at += escape
		}
		at++
	}
	return segmentAt, len(rest), 0, found
}

// bracketClosesInSegment reports whether the bracket body starting at at is
// closed by a `]` before the segment ends, the test the shell's expansion
// uses to decide whether a `[` is a wildcard at all.
func bracketClosesInSegment(rest string, at int) bool {
	if at < len(rest) && rest[at] == '!' {
		at++
	}
	for {
		if at < len(rest) && rest[at] == '\\' {
			at++
		}
		if at >= len(rest) || rest[at] == '/' {
			return false
		}
		at++
		if at < len(rest) && rest[at] == ']' {
			return true
		}
	}
}

// segmentHasWildcard reports whether one segment holds a wildcard by the
// shell's scan.
func segmentHasWildcard(segment string) bool {
	_, _, _, found := findWildcardSegment(segment)
	return found
}

// unescape removes the backslash before each escaped byte; a trailing
// backslash stays, as the shell keeps it.
func unescape(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	var plain strings.Builder
	for at := 0; at < len(text); at++ {
		if text[at] == '\\' && at+1 < len(text) {
			at++
		}
		plain.WriteByte(text[at])
	}
	return plain.String()
}
