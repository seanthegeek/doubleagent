// Package glob matches and expands shell wildcard patterns ("globs") the way
// the reference shell, dash, does under LC_ALL=C: byte by byte, with `*`, `?`,
// bracket expressions and `\` escaping. MatchPath is the shell's `case` test,
// MatchSegment one step of pathname expansion, and Expand the whole expansion
// of a pattern inside a base directory; "Glob matching" in
// collectors/docs/internals.md says which table each one serves. The tables
// hold only portable patterns: `*`, `?` and a closed bracket list with literal
// bytes, ranges, `!` or a character class. On an unclosed bracket, a backslash,
// a caret, a collating or equivalence class or a class beside a range the
// shells disagree with one another, and the result here is one shell's.
package glob

import "strings"

// MatchPath reports whether path matches pattern as the shell's `case` does:
// `*` and `?` match any byte, `/` and newline included, and a leading `.`
// needs no literal dot.
func MatchPath(pattern, path string) bool {
	return match(pattern, path)
}

// MatchSegment reports whether one directory entry name matches one segment
// of a pathname pattern. A segment with no wildcard is compared as literal
// text with its escapes removed, as the shell appends such a segment
// untested; otherwise a name starting with `.` matches only a segment whose
// first character, escaped or not, is a dot.
func MatchSegment(pattern, name string) bool {
	if !segmentHasWildcard(pattern) {
		return unescape(pattern) == name
	}
	if strings.HasPrefix(name, ".") && !segmentMatchesDot(pattern) {
		return false
	}
	return match(pattern, name)
}

// QuoteMeta returns text with `[`, `]`, `*`, `?` and `\` escaped by a
// backslash, so a pattern built from it matches text literally; it is the
// escaping the sh collector applies to a base path before using it in a
// pattern.
func QuoteMeta(text string) string {
	var quoted strings.Builder
	for index := 0; index < len(text); index++ {
		if strings.IndexByte(`[]*?\`, text[index]) >= 0 {
			quoted.WriteByte('\\')
		}
		quoted.WriteByte(text[index])
	}
	return quoted.String()
}

// segmentMatchesDot reports whether a segment may match a name starting with
// `.`: the shell lets it when the segment's first character, after one
// optional backslash, is a dot.
func segmentMatchesDot(segment string) bool {
	segment = strings.TrimPrefix(segment, `\`)
	return strings.HasPrefix(segment, ".")
}

// match is the shell's pattern test over bytes, with the usual backtracking
// for `*`: on a mismatch the last `*` takes one more byte and the match
// resumes after it. A malformed construct the shell rejects (an unclosed
// bracket, an unknown class, a trailing backslash) fails the whole test,
// since every retry reaches it at the same place in the pattern. The shell
// puts a backslash before every `^` first, so `[^a]` holds `^` and `\^`
// matches a backslash and a caret.
func match(pattern, subject string) bool {
	pattern = strings.ReplaceAll(pattern, "^", `\^`)
	patternAt, subjectAt := 0, 0
	starPattern, starSubject := -1, 0
	for {
		if patternAt == len(pattern) {
			if subjectAt == len(subject) {
				return true
			}
		} else if pattern[patternAt] == '*' {
			patternAt++
			starPattern, starSubject = patternAt, subjectAt
			continue
		} else {
			next, outcome := step(pattern, patternAt, subject, subjectAt)
			if outcome == stepInvalid {
				return false
			}
			if outcome == stepMatched {
				patternAt, subjectAt = next, subjectAt+1
				continue
			}
		}
		if starPattern < 0 || starSubject >= len(subject) {
			return false
		}
		starSubject++
		patternAt, subjectAt = starPattern, starSubject
	}
}

// stepOutcome is what one pattern element did against one subject byte.
type stepOutcome int

const (
	stepMatched stepOutcome = iota
	stepMismatched
	stepInvalid
)

// step matches the pattern element at patternAt (not `*`) against the subject
// byte at subjectAt and returns the index after the element.
func step(pattern string, patternAt int, subject string, subjectAt int) (int, stepOutcome) {
	element := pattern[patternAt]
	if element == '\\' && patternAt+1 == len(pattern) {
		return 0, stepInvalid
	}
	if subjectAt == len(subject) {
		return 0, stepMismatched
	}
	char := subject[subjectAt]
	switch element {
	case '?':
		return patternAt + 1, stepMatched
	case '[':
		return matchBracket(pattern, patternAt+1, char)
	case '\\':
		patternAt++
	}
	if pattern[patternAt] != char {
		return 0, stepMismatched
	}
	return patternAt + 1, stepMatched
}
