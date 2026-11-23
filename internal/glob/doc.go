// Package glob matches and expands shell wildcard patterns ("globs") the way
// the reference shell, dash, does under LC_ALL=C: byte by byte, with `*`, `?`,
// bracket expressions and `\` escaping. MatchPath is the shell's `case` test,
// MatchSegment one step of pathname expansion, and Expand the whole expansion
// of a pattern inside a base; "Glob matching" in docs/internals.md
// says which table each serves. The tables hold only `*`, `?` and a closed
// bracket list of literal bytes, ranges and `!`: never a character class name
// such as [:alpha:], which the catalog rules of internal/catalog/AGENTS.md and
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
// ASCII case is ignored, where filepath.Match is case-sensitive, and
// MatchPathFolding ignores it in the path bytes its caller says fold: the
// exclusion and secret tests, in the components whose directory folds case
// on Linux and macOS. Expand also
// asks the image root's check before it lists a directory and reports each
// link it refused, and reads the disk only through osfile and fsmeta, where
// filepath.Glob follows links freely through os.Stat and os.Open.
package glob
