// Package replay replays the corpus the Python analyzer's recorder writes
// (analyzer/tests/recorder, whose package docstring documents the format)
// against the Go packages. Its files, all but this one test files with the
// parity build tag, read the corpus PARITY_CORPUS names (a test skips when it
// is unset), recreate a case's tree under the test's temporary directory with
// its modes, times and links, and run each recorded parser call and command.
//
// The comparison is of information, not bytes: JSON outputs and embedded
// JSON in one canonical form (members sorted by key, a key written twice
// kept twice, numbers as written), the CSV as the records encoding/csv reads
// with its blank lines, stdout as written, and a row's text through one
// canonical form. Each rule the binary follows in place of the analyzer's is
// a numbered difference with its own narrow rule and test. No case names
// one: a difference of text, rows or arguments applies wherever its shape
// shows in the recorded text, an excluding one leaves out a case whose
// recorded text shows its shape, and a difference of format applies to the
// texts its scope names. The log counts the cases that used each, and a run
// of every case fails when a difference that is not retired was used by
// none. Mutation tests show that a changed word, number, key, row order,
// line order or dropped row still fails.
//
// The recorder ran under a temporary root padded to a fixed length and wrote
// a placeholder of that length wherever the root appeared, file contents
// included. Materialise pads its own root to the same length, so putting it
// back where the placeholder stands moves no offset inside a binary file, and
// Expand does the same for the text of a case.
package replay
