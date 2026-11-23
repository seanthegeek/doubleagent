// Package analyze implements the analysis commands of the binary: detect
// (the homes, users and agents a collection holds), timeline (every
// transcript parsed into timeline.jsonl, sessions.jsonl and inventory.json),
// fleet (saved outputs of collect --inventory merged into one CSV) and
// catalog (the bundled artifact catalog).
//
// Each command reads its options with its own flag set, and Run maps what a
// run found to its exit code: 0 when it found something, 1 when it found
// nothing, 2 for a command-line mistake or an error, 130 when it was
// interrupted. Every output file and every table has a fixed field order
// (a struct's for the JSON files, the column list for the CSV), so analysts
// can build on them.
package analyze
