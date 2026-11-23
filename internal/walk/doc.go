// Package walk is the catalog walk: it expands the catalog tables under a
// base (a home, a project or a volume), folds case variants of one path,
// claims nested matches, walks each match depth first and stages every file
// with its manifest row. docs/internals.md, "Glob matching" and
// "Claimed paths and case variants", has the rules.
//
// filepath.WalkDir and os.ReadDir return a directory's entries sorted by name
// and know nothing of the image root; this walk lists each directory in the
// order the system gives it and asks the root check before it enters one.
package walk
