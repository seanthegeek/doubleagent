package pi

import (
	"iter"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// Entry is one record of a session file after its header, with its line.
type Entry struct {
	Line   int
	Record pyjson.Value
}

// Session is one pi-format session file read whole: its header, the
// entries after it and the problems met while reading it. Letta's local
// backend writes the same format.
type Session struct {
	// Header is the first record when its type is "session", else null.
	Header pyjson.Value
	// HeaderLine is the header's line number, 0 when there is none.
	HeaderLine int
	Entries    []Entry
	Problems   parser.Problems
}

// HasHeader reports whether the session has a header record.
func (s *Session) HasHeader() bool {
	return s.Header.IsObject()
}

// ReadSession reads a pi-format session file into its header, entries and
// bad lines, which go to the session's Problems.
func ReadSession(path string) *Session {
	session := &Session{}
	session.fill(parser.JSONLines(path, &session.Problems))
	return session
}

// ReadSessionBytes reads a pi-format session held in memory, as ReadSession
// reads a file.
func ReadSessionBytes(data []byte) *Session {
	session := &Session{}
	session.fill(parser.JSONRecords(parser.SplitLines(data), &session.Problems))
	return session
}

// fill takes the first record as the header when its type is "session" and
// every other record as an entry.
func (s *Session) fill(records iter.Seq2[int, pyjson.Value]) {
	first := true
	for line, record := range records {
		if first {
			first = false
			if isText(record.Get("type"), "session") {
				s.Header, s.HeaderLine = record, line
				continue
			}
		}
		s.Entries = append(s.Entries, Entry{Line: line, Record: record})
	}
}

// entryKey is the identity of an entry a fork copies from its parent: its
// id and its timestamp as text.
type entryKey struct {
	identifier, timestamp string
}

// keyOf returns the identity of an entry record.
func keyOf(record pyjson.Value) entryKey {
	return entryKey{identifier: parser.Str(record.Get("id")),
		timestamp: parser.Str(record.Get("timestamp"))}
}

// entryKeys returns the identity of every entry of the session file at path
// that has an id; the file's own problems are not reported here.
func entryKeys(path string) map[entryKey]bool {
	keys := map[entryKey]bool{}
	for _, entry := range ReadSession(path).Entries {
		if entry.Record.Get("id").Truthy() {
			keys[keyOf(entry.Record)] = true
		}
	}
	return keys
}

// isText reports whether value is the string text, as Python's == compares
// a decoded value with a string.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}
