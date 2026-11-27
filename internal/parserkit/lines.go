package parserkit

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"iter"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// errNotObject is the reason a JSON line holding something other than an
// object is skipped.
var errNotObject = errors.New("not a JSON object")

// Lines yields (line number, raw line) for every line of the file at path,
// numbered from 1, each with its terminator: a line ends at LF, so a CRLF
// line keeps its CR, and a last line without LF is yielded as it stands. A
// file that cannot be opened adds its unreadable problem at line 0; one that
// fails while being read adds it at the last line read, after the lines
// before it. The file is closed when the loop ends, early or not.
func Lines(path string, problems *Problems) iter.Seq2[int, []byte] {
	return func(yield func(int, []byte) bool) {
		file, err := Open(path)
		if err != nil {
			problems.AddUnreadable(0, path, err)
			return
		}
		defer file.Close()
		reader := bufio.NewReader(file)
		for number := 1; ; number++ {
			raw, err := reader.ReadBytes('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				problems.AddUnreadable(number-1, path, err)
				return
			}
			if len(raw) > 0 && !yield(number, raw) {
				return
			}
			if err != nil {
				return
			}
		}
	}
}

// SplitLines yields (line number, raw line) for the lines of data held in
// memory, as Lines yields those of a file.
func SplitLines(data []byte) iter.Seq2[int, []byte] {
	return func(yield func(int, []byte) bool) {
		rest := data
		for number := 1; len(rest) > 0; number++ {
			end := bytes.IndexByte(rest, '\n') + 1
			if end == 0 {
				end = len(rest)
			}
			if !yield(number, rest[:end]) {
				return
			}
			rest = rest[end:]
		}
	}
}

// JSONRecords yields (line number, record) for each line of lines that
// holds a JSON object. A blank line is skipped. A line that does not decode,
// or decodes to something other than an object, adds a problem at its
// number with the line as raw text (LineText) and is skipped, so a
// transcript cut mid-write keeps its earlier records. A line is stripped of
// ASCII whitespace and decoded as UTF-8 with replacement before it is read.
// The records that repeat a member name of an object give one problem
// (repeatedMembers) at the first such line, added when the loop ends.
func JSONRecords(lines iter.Seq2[int, []byte], problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return JSONRecordsIn(nil, lines, problems)
}

// JSONRecordsIn yields the records of lines as JSONRecords does, for lines
// held in one of many values of a file (a database row's blob): their
// repeated member names are noted in the parse of artifact (NoteRepeats),
// so the file gives one row however many of its values repeat one. A nil
// artifact gives JSONRecords' one problem instead.
func JSONRecordsIn(artifact *input.Artifact, lines iter.Seq2[int, []byte],
	problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return func(yield func(int, jsonvalue.Value) bool) {
		reader := &JSONLineReader{problems: problems, artifact: artifact}
		defer reader.Close()
		for number, raw := range lines {
			if record, ok := reader.Record(number, raw); ok && !yield(number, record) {
				return
			}
		}
	}
}

// JSONLineReader reads the JSON object lines of one file handed to it one
// at a time, as JSONRecords reads them, for a reader whose lines arrive
// from a stream it does not pull (decompressed text): Close, when the
// file's lines end, adds the file's one problem of repeated member names.
type JSONLineReader struct {
	problems *Problems
	// artifact, when set, is the parse the repeats are noted in, in place
	// of the problem (JSONRecordsIn).
	artifact *input.Artifact
	noted    repeated
}

// NewJSONLineReader returns the reader of one file's lines, adding its
// problems to problems.
func NewJSONLineReader(problems *Problems) *JSONLineReader {
	return &JSONLineReader{problems: problems}
}

// Record returns the JSON object of the line raw at number and true, or
// false for a blank line and for one that adds its problem, as JSONRecords
// reads a line.
func (r *JSONLineReader) Record(number int, raw []byte) (jsonvalue.Value, bool) {
	stripped := bytes.Trim(raw, textlines.ASCIIWhitespace)
	if len(stripped) == 0 {
		return jsonvalue.Value{}, false
	}
	document := jsonvalue.NewDocument(string(stripped))
	record, err := document.Decode()
	if err == nil && !record.IsObject() {
		err = errNotObject
	}
	if err != nil {
		r.problems.AddRaw(number, UnparseableLine(err), LineText(raw))
		return jsonvalue.Value{}, false
	}
	r.noted.note(number, document.TakeRepeats())
	return record, true
}

// Close adds the problem of the repeated member names the lines noted, if
// any, at the first line that repeated one.
func (r *JSONLineReader) Close() {
	if r.artifact != nil {
		NoteRepeats(r.artifact, r.noted.repeats)
	} else {
		r.noted.report(r.problems)
	}
	r.noted = repeated{}
}

// JSONLines yields (line number, record) for each JSON object line of the
// file at path: the lines of Lines read as JSONRecords reads them.
func JSONLines(path string, problems *Problems) iter.Seq2[int, jsonvalue.Value] {
	return JSONRecords(Lines(path, problems), problems)
}

// LineText returns a raw line as the text of an error row: without its
// trailing LF and then CR, nothing else, decoded as UTF-8 with replacement.
func LineText(raw []byte) string {
	raw = bytes.TrimSuffix(raw, []byte("\n"))
	raw = bytes.TrimSuffix(raw, []byte("\r"))
	return textlines.ValidUTF8(raw)
}

// TailText returns text from the byte offset start to its end as the text
// of an error row, without a trailing LF and then CR. A JSON array reader
// passes the offset of the record it could not decode: a broken record has
// no end of its own, so everything after it goes too.
func TailText(text string, start int) string {
	tail := text[min(max(start, 0), len(text)):]
	tail = strings.TrimSuffix(tail, "\n")
	return strings.TrimSuffix(tail, "\r")
}
