package glob

// FoldingPattern is a MatchPath pattern kept beside its ASCII-lowered form.
// Lowering keeps every index and every bracket's structure, so a match can
// read each subject byte against either form at the same place.
type FoldingPattern struct {
	raw     string
	lowered string
}

// NewFoldingPattern returns pattern with its lowered form, made once so a
// table of patterns is lowered once for a whole walk.
func NewFoldingPattern(pattern string) FoldingPattern {
	raw := escapeCarets(pattern)
	return FoldingPattern{raw: raw, lowered: foldASCII(raw)}
}

// MatchPathFolding reports whether path matches pattern as MatchPath does on
// Linux and macOS, except that the path byte at each index for which folds
// is true is compared ignoring ASCII case: lowered, against the lowered
// pattern ([A-Z] reads as [a-z] there).
func MatchPathFolding(pattern, path string, folds func(index int) bool) bool {
	return NewFoldingPattern(pattern).Match(path, folds)
}

// Match is MatchPathFolding for the pattern.
func (pattern FoldingPattern) Match(path string, folds func(index int) bool) bool {
	return backtrack(pattern.raw, len(path), func(patternAt, subjectAt int) (int, stepOutcome) {
		if subjectAt < len(path) && folds(subjectAt) {
			return step(pattern.lowered, patternAt, path, subjectAt, true)
		}
		return step(pattern.raw, patternAt, path, subjectAt, false)
	})
}

// MatchEitherCase reports whether some choice of folds could make Match
// true: each byte is accepted when the byte comparison or the folded one
// accepts it. Neither the byte nor the fully folded match alone is that
// bound, since `[!a]` takes `A` only by byte and a path can need both forms.
func (pattern FoldingPattern) MatchEitherCase(path string) bool {
	return backtrack(pattern.raw, len(path), func(patternAt, subjectAt int) (int, stepOutcome) {
		next, outcome := step(pattern.raw, patternAt, path, subjectAt, false)
		if outcome == stepMatched {
			return next, outcome
		}
		foldedNext, foldedOutcome := step(pattern.lowered, patternAt, path, subjectAt, true)
		if foldedOutcome == stepMatched || outcome == stepInvalid && foldedOutcome == stepInvalid {
			return foldedNext, foldedOutcome
		}
		return 0, stepMismatched
	})
}

// MatchBytes is MatchPath on Linux and macOS: every byte compared as written.
func (pattern FoldingPattern) MatchBytes(path string) bool {
	return backtrack(pattern.raw, len(path), func(patternAt, subjectAt int) (int, stepOutcome) {
		return step(pattern.raw, patternAt, path, subjectAt, false)
	})
}
