// Package glob matches and expands shell wildcard patterns ("globs") the way
// the reference shell, dash, does under LC_ALL=C: byte by byte, with `*`, `?`,
// bracket expressions and `\` escaping. MatchPath is the shell's `case` test,
// MatchSegment one step of pathname expansion, and Expand the whole expansion
// of a pattern inside a base; "Glob matching" in docs/internals.md
// says which table each serves. The tables hold only `*`, `?` and a closed
// bracket list of literal bytes, ranges and `!`: never a character class name
// such as [:alpha:], which the AGENTS.md catalog rule and
// internal/catalog/rules_test.go refuse. On a class name, an unclosed bracket, a backslash, a caret or a
// collating or equivalence class the shells disagree; the result is one's.
//
// The standard library's path.Match, filepath.Match and filepath.Glob read a
// different pattern language, so they cannot stand in for this package. There
// `*` and `?` stop at `/` and match characters (UTF-8 runes), a leading dot
// is no special case, negation is `[^...]` and a malformed pattern is an
// error. Here MatchPath lets `*` cross `/`, matching is by byte, so a name
// that is not valid UTF-8 matches predictably, MatchSegment and Expand let a
// name starting with a dot match only a segment that starts with a dot,
// negation is `[!...]` (a caret is an ordinary character), and no pattern is
// an error: an unclosed
// `[` is plain text in Expand and fails the match in MatchPath. On Windows
// ASCII case is ignored, where filepath.Match is case-sensitive. Expand also
// asks the image root's check before it lists a directory and reports each
// link it refused, and reads the disk only through osfile and fsmeta, where
// filepath.Glob follows links freely through os.Stat and os.Open.
package glob

import "strings"

// MatchPath reports whether path matches pattern as the shell's `case` does:
// `*` and `?` match any byte, `/` and newline included, and a leading `.`
// needs no literal dot. On Windows every test in this package ignores ASCII
// case, pattern and subject lowered first; a bracket range is lowered as written ([A-Z] is [a-z]).
func MatchPath(pattern, path string) bool {
	return match(fold(pattern), fold(path))
}

// MatchSegment reports whether one directory entry name matches one segment
// of a pathname pattern. A segment with no wildcard is compared as literal
// text with its escapes removed, as the shell appends such a segment
// untested; otherwise a name starting with `.` matches only a segment whose
// first character, escaped or not, is a dot.
func MatchSegment(pattern, name string) bool {
	return matchSegment(pattern, name, fold)
}

// matchSegment is MatchSegment with names compared after foldName, so a
// test can give it the fold of another platform.
func matchSegment(pattern, name string, foldName func(string) string) bool {
	if !segmentHasWildcard(pattern) {
		return foldName(unescape(pattern)) == foldName(name)
	}
	return wildcardMatches(pattern, name, foldName)
}

// wildcardMatches reports whether name matches a segment holding a
// wildcard: a name starting with `.` only when the segment's first
// character, escaped or not, is a dot, and then by match over the folded
// forms. The dot rule reads the names as written, since folding never
// changes a dot.
func wildcardMatches(segment, name string, foldName func(string) string) bool {
	if strings.HasPrefix(name, ".") && !segmentMatchesDot(segment) {
		return false
	}
	return match(foldName(segment), foldName(name))
}

// foldASCII returns text with the ASCII capitals A to Z lowered and every
// other byte kept, the Windows fold; it is plain code so every platform
// tests it.
func foldASCII(text string) string {
	lowered := []byte(text)
	for i, character := range lowered {
		if 'A' <= character && character <= 'Z' {
			lowered[i] = character + 'a' - 'A'
		}
	}
	return string(lowered)
}

// QuoteMeta returns text with `[`, `]`, `*`, `?` and `\` escaped by a
// backslash, so a pattern built from it matches text literally; a base path
// is escaped so before it is used in a pattern.
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
