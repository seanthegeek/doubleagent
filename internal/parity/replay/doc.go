// Package replay replays the corpus the Python analyzer's recorder writes
// (analyzer/tests/recorder, whose package docstring documents the format)
// against the Go packages. Its files, all but this one test files with the
// parity build tag, read the corpus PARITY_CORPUS names (a test skips when it
// is unset), recreate a case's tree under the test's temporary directory with
// its modes, times and links, and run each recorded parser call and command.
//
// The comparison is of information, not bytes: JSON outputs are compared as
// decoded values (object keys in any order, numbers by value), the CSV as the
// records encoding/csv reads, stdout tables by column, and a row's text
// through one canonical form. Each formatting rule the binary follows in
// place of the analyzer's (true for True, an empty value for None, a number
// as the evidence wrote it, embedded JSON compact in the evidence's order) is
// a numbered difference with its own narrow rule and test, and the replay
// logs how many cases needed each. A difference named for a case that finds
// nothing to rewrite fails the case, and mutation tests show that a changed
// word, number, key, row order or dropped row still fails.
//
// The recorder ran under a temporary root padded to a fixed length and wrote
// a placeholder of that length wherever the root appeared, file contents
// included. Materialise pads its own root to the same length, so putting it
// back where the placeholder stands moves no offset inside a binary file, and
// Expand does the same for the text of a case.
package replay
