// Package replay replays the corpus the Python analyzer's recorder writes
// (analyzer/tests/recorder, whose package docstring documents the format)
// against the Go packages. Its files, all but this one test files with the
// parity build tag, read the corpus PARITY_CORPUS names (a test skips when it
// is unset), recreate a case's tree under the test's temporary directory with
// its modes, times and links, and run each recorded parser call and command.
//
// The comparison is of information, not bytes: JSON outputs and embedded JSON
// in one canonical form (members sorted by key, a key written twice kept
// twice, numbers as written), the CSV as the records encoding/csv reads with
// its blank lines, stdout as written, and a row's text through one canonical
// form. Each rule the binary follows in place of the analyzer's is a numbered
// difference with its own narrow rule and test, named by no case: a difference
// of text, rows or arguments applies wherever its shape shows in the recorded
// text, rewriting the recorded wording on the recording's side and the
// binary's own on the binary's, so the binary printing the recorded wording
// still fails; an excluding one leaves out a case whose recorded text shows
// its shape, a difference of format applies to the texts its scope names, one
// of outputs lets a loose tree's run keep the other of two equal rows, and a
// wanted one names a file a parser may want that the recording's did not.
// A difference of format, text or rows counts as used by a case that fails
// without it; one of arguments, outputs, wanted files, added rows or exclusion
// by a case it changed. A run of every case fails when one was used by none,
// unless it is retired or marked unchecked with its reason. A loose tree's
// timeline is compared sorted by its documented keys, a record's rows in their
// order; every timeline and sessions file, run or recorded, is checked for the
// documented order. Mutation tests show a changed word, number, key or order,
// or a dropped row, still fails. The recorder wrote a fixed-length placeholder
// for its root, file contents included; Materialise and Expand put back a root
// of that length, so no binary offset moves.
package replay
