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
// for `*` (backtrack). The shell puts a backslash before every `^` first, so
// `[^a]` holds `^` and `\^` matches a backslash and a caret.
func match(pattern, subject string) bool {
	pattern = escapeCarets(pattern)
	return backtrack(pattern, len(subject), func(patternAt, subjectAt int) (int, stepOutcome) {
		return step(pattern, patternAt, subject, subjectAt, false)
	})
}

// escapeCarets returns pattern with a backslash before every `^`, as the
// shell reads a pattern. Lowering changes no `^`, so a pattern and its
// ASCII-lowered form escaped so still line up byte for byte.
func escapeCarets(pattern string) string {
	return strings.ReplaceAll(pattern, "^", `\^`)
}

// backtrack runs the shell's match of pattern against a subject of
// subjectLength bytes: compare tests the pattern element at patternAt (not
// `*`) against the subject byte at subjectAt. On a mismatch the last `*`
// takes one more byte and the match resumes after it. A malformed construct
// the shell rejects (an unclosed bracket, an unknown class, a trailing
// backslash) fails the whole test, since every retry reaches it at the same
// place in the pattern.
func backtrack(pattern string, subjectLength int,
	compare func(patternAt, subjectAt int) (int, stepOutcome),
) bool {
	patternAt, subjectAt := 0, 0
	starPattern, starSubject := -1, 0
	for {
		if patternAt == len(pattern) {
			if subjectAt == subjectLength {
				return true
			}
		} else if pattern[patternAt] == '*' {
			patternAt++
			starPattern, starSubject = patternAt, subjectAt
			continue
		} else {
			next, outcome := compare(patternAt, subjectAt)
			if outcome == stepInvalid {
				return false
			}
			if outcome == stepMatched {
				patternAt, subjectAt = next, subjectAt+1
				continue
			}
		}
		if starPattern < 0 || starSubject >= subjectLength {
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
// byte at subjectAt, lowered first when lower is set, and returns the index
// after the element.
func step(pattern string, patternAt int, subject string, subjectAt int, lower bool,
) (int, stepOutcome) {
	element := pattern[patternAt]
	if element == '\\' && patternAt+1 == len(pattern) {
		return 0, stepInvalid
	}
	if subjectAt == len(subject) {
		return 0, stepMismatched
	}
	char := subject[subjectAt]
	if lower && 'A' <= char && char <= 'Z' {
		char += 'a' - 'A'
	}
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
