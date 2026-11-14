package glob

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Expand returns base + "/" + match for every match of pattern inside base,
// an absolute path or empty for the root, never itself a pattern, as the sh
// collector's `cd -P BASE` and unquoted `for` loop give them: the pattern is
// split into words at newlines, as with IFS set to newline only; each word's
// matches are sorted bytewise; a word that matches nothing is kept as typed,
// escapes and all, when an entry by that name exists, and dropped otherwise.
// Directories are read as the shell reads them, so a symlinked directory in
// the middle of the pattern is followed. On Windows each existing literal
// segment comes back in on-disk case, and a word's matches are not sorted
// but come in Get-ChildItem's order, as PowerShell's Expand-Glob gives them.
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
			if exists(base + "/" + word) {
				results = append(results, base+"/"+respell(base, word))
			}
			continue
		}
		sortMatches(matches)
		for _, match := range matches {
			results = append(results, base+"/"+match)
		}
	}
	return results
}

// expandFrom appends to matches each path, relative to base, that extends
// prefix (empty or ending in `/`) by a match of rest. Literal segments
// before the first wildcard segment are appended untested; the wildcard
// segment's directory is read; a literal tail after the last wildcard
// segment is kept only when exists finds it. Literal segments pass
// through respell, which on Windows gives their on-disk case. A whole
// pattern with no wildcard segment gives nothing here: Expand falls back to
// the word itself.
func expandFrom(base, prefix, rest string, matches *[]string) {
	segmentAt, segmentEnd, separatorWidth, found := findWildcardSegment(rest)
	if !found {
		if prefix == "" {
			return
		}
		tail := unescape(rest)
		if exists(base + "/" + prefix + tail) {
			*matches = append(*matches, prefix+respell(base+"/"+prefix, tail))
		}
		return
	}
	prefix += respell(base+"/"+prefix, unescape(rest[:segmentAt]))
	segment, atEnd := rest[segmentAt:segmentEnd], segmentEnd == len(rest)
	for _, name := range readNames(base+"/"+prefix, segmentMatchesDot(segment)) {
		if !wildcardMatches(segment, name, fold) {
			continue
		}
		if atEnd {
			*matches = append(*matches, prefix+name)
		} else {
			expandFrom(base, prefix+name+"/", rest[segmentEnd+separatorWidth:], matches)
		}
	}
}

// exists reports whether path names an entry, as the shell's lstat of it
// answers. A path ending in a slash must name a directory or a link to one,
// which is decided by following the path without its slash rather than by
// how the platform reads a trailing separator: Windows has been seen to let
// a file through as `name\`.
func exists(path string) bool {
	if trimmed, isDirectory := strings.CutSuffix(path, "/"); isDirectory {
		info, err := osfile.Stat(trimmed)
		return err == nil && info.IsDir()
	}
	_, err := osfile.Lstat(path)
	return err == nil
}

// readNames returns the entry names of directory in the platform's listing
// order, with `.` and `..` added first when withDots is set on a platform
// whose directory read lists them (listsDotEntries), since a segment starting
// with a dot matches them there. A directory that cannot be read gives the names read before
// the error, as the shell's read loop stops.
func readNames(directory string, withDots bool) []string {
	handle, err := osfile.Open(directory)
	if err != nil {
		return nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	names := make([]string, 0, len(entries)+2)
	if withDots && listsDotEntries {
		names = append(names, ".", "..")
	}
	for _, entry := range fsmeta.ListingOrder(entries) {
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
	for position := 0; position < len(rest); {
		switch rest[position] {
		case '*', '?':
			found = true
		case '[':
			found = found || bracketClosesInSegment(rest, position+1)
		default:
			escape := 0
			if rest[position] == '\\' && position+1 < len(rest) {
				escape = 1
			}
			if rest[position+escape] == '/' {
				if found {
					return segmentAt, position, escape + 1, true
				}
				segmentAt = position + escape + 1
			}
			position += escape
		}
		position++
	}
	return segmentAt, len(rest), 0, found
}

// bracketClosesInSegment reports whether the bracket body starting at position is
// closed by a `]` before the segment ends, the test the shell's expansion
// uses to decide whether a `[` is a wildcard at all.
func bracketClosesInSegment(rest string, position int) bool {
	if position < len(rest) && rest[position] == '!' {
		position++
	}
	for {
		if position < len(rest) && rest[position] == '\\' {
			position++
		}
		if position >= len(rest) || rest[position] == '/' {
			return false
		}
		position++
		if position < len(rest) && rest[position] == ']' {
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
	for position := 0; position < len(text); position++ {
		if text[position] == '\\' && position+1 < len(text) {
			position++
		}
		plain.WriteByte(text[position])
	}
	return plain.String()
}
