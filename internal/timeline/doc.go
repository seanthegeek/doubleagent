// Package timeline defines the rows of timeline.jsonl and sessions.jsonl:
// the Row every parser emits, the per-session summary built from the rows,
// the marker texts a parser starts a system row with, the spellings of a
// Windows project path, the stable sorts, the JSONL writers, and the long
// texts that several rows name, written in full once (Resolve).
//
// Both files are interfaces analysts build on: fields are added at the end,
// never renamed or removed. The writers put each record on one line, keys
// in column order, non-ASCII text as UTF-8, U+2028 and U+2029 escaped since
// some readers split lines at them, and each byte of invalid UTF-8 as
// U+FFFD.
package timeline
