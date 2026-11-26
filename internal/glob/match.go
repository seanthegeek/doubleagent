package glob

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// MatchPath reports whether path matches pattern as a whole: `*` matches any
// run of bytes and `?` any one byte, `/` included, and a leading `.` needs
// no literal dot.
func MatchPath(pattern, path string) bool {
	return match(pattern, path)
}

// MatchSegment reports whether one directory entry name matches one segment
// of a pathname pattern, as MatchPath does, except that a name starting with
// `.` matches only a segment starting with `.`, so a wildcard never takes a
// hidden name the catalog did not ask for.
func MatchSegment(segment, name string) bool {
	if strings.HasPrefix(name, ".") && !strings.HasPrefix(segment, ".") {
		return false
	}
	return match(segment, name)
}

// match is the pattern test over bytes, ASCII case ignored, with the usual
// backtracking for `*`: on a mismatch the last `*` takes one more byte and
// the match resumes after it. A bracket that is not well formed fails the
// whole test, since every retry reaches it at the same place. Only ASCII is
// folded, so no match takes as one two names NTFS keeps apart (syspath.Fold
// says why).
func match(pattern, subject string) bool {
	patternAt, subjectAt := 0, 0
	starPattern, starSubject := -1, 0
	for {
		if patternAt < len(pattern) && pattern[patternAt] == '*' {
			patternAt++
			starPattern, starSubject = patternAt, subjectAt
			continue
		}
		if patternAt == len(pattern) && subjectAt == len(subject) {
			return true
		}
		if patternAt < len(pattern) && subjectAt < len(subject) {
			next, matched, valid := element(pattern, patternAt, textlines.LowerByte(subject[subjectAt]))
			if !valid {
				return false
			}
			if matched {
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

// element tests the pattern element at position (not `*`) against char,
// already lowered, and returns the index after it, whether it matched and
// whether it is well formed.
func element(pattern string, position int, char byte) (int, bool, bool) {
	switch pattern[position] {
	case '?':
		return position + 1, true, true
	case '[':
		return bracket(pattern, position+1, char)
	}
	return position + 1, textlines.LowerByte(pattern[position]) == char, true
}

// bracket tests char against the list whose body starts at position, just
// after its `[`: an optional `!` that negates it, then bytes and `x-y`
// ranges, lowered as written ([A-Z] is [a-z]), up to the closing `]`; a `]`
// first in the list is a member. A range is over lowered bytes, so `[Z-a]`,
// lowered to `[z-a]`, matches nothing. A list with no closing `]` or holding
// a `[`, which would open a class name, is not well formed.
func bracket(pattern string, position int, char byte) (int, bool, bool) {
	negate := position < len(pattern) && pattern[position] == '!'
	if negate {
		position++
	}
	matched := false
	for first := position; position < len(pattern); position++ {
		low := pattern[position]
		if low == ']' && position > first {
			return position + 1, matched != negate, true
		}
		if low == '[' {
			return 0, false, false
		}
		high := low
		if position+2 < len(pattern) && pattern[position+1] == '-' && pattern[position+2] != ']' {
			high = pattern[position+2]
			position += 2
		}
		matched = matched || textlines.LowerByte(low) <= char && char <= textlines.LowerByte(high)
	}
	return 0, false, false
}
