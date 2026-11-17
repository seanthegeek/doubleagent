// Package replay replays the corpus the Python analyzer's recorder writes
// (analyzer/tests/recorder, whose package docstring documents the format)
// against the Go packages. Its files, all but this one test files with the
// parity build tag, read the corpus PARITY_CORPUS names (a test skips when it
// is unset), recreate a case's tree under the test's temporary directory with
// its modes, times and links, and compare rows and texts after the numbered
// documented differences are normalised. The tests of each parser and command
// are added beside these helpers.
//
// The recorder ran under a temporary root padded to a fixed length and wrote
// a placeholder of that length wherever the root appeared, file contents
// included. Materialise pads its own root to the same length, so putting it
// back where the placeholder stands moves no offset inside a binary file, and
// Expand does the same for the text of a case.
package replay
