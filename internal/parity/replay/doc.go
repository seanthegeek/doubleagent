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
// its shape, and a difference of format applies to the texts its scope names.
// A difference of format, text or rows counts as used by a case that fails
// without it; one of arguments, added rows or exclusion by a case it changed.
// A run of every case fails when a difference was used by none, unless it is
// retired or marked unchecked with its reason. A loose tree's timeline is
// compared sorted by its documented keys, the rows of one record in their
// order, and its documented order is checked on its own. Mutation tests show that a
// changed word, number, key, row order, line order or dropped row still fails.
//
// The recorder wrote a placeholder of fixed length wherever its root appeared,
// file contents included; Materialise and Expand put back a root of the same
// length, so no offset inside a binary file moves.
package replay
