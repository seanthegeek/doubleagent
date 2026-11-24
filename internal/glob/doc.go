// Package glob matches and expands the wildcard patterns ("globs") of the
// catalog tables: `*`, `?`, and bracket lists `[...]` and `[!...]` of bytes
// and ranges, matched byte by byte with ASCII case ignored on every
// platform; any other form matches nothing, and the catalog rules test
// refuses it. MatchPath tests a whole path (its `*` crosses `/`),
// MatchSegment one directory entry name, and Expand finds a pattern's
// matches inside a base; "Glob matching" in docs/internals.md says which
// table each serves.
//
// The standard library's path.Match, filepath.Match and filepath.Glob read
// another language: `*` and `?` stop at `/` and match UTF-8 characters,
// case counts outside Windows, a leading dot is no special case and
// negation is `[^...]`. Here matching is by byte, so a name that is not
// valid UTF-8 matches predictably, and case never counts, so one catalog
// line reaches a directory however an install or a mount spells it. Expand
// compares each segment with the names its directory lists, so a match is
// spelled as the disk holds it, asks the image root's check before it lists
// a directory and reports each link it refused, and reads the disk only
// through osfile and fsmeta, where filepath.Glob follows links freely.
package glob
