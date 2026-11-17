package parser

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"iter"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// errNotObject is the reason a JSON line holding something other than an
// object is skipped.
var errNotObject = errors.New("not a JSON object")

// asciiWhitespace is what Python's bytes.strip() removes from both ends of
// a line: space, tab, LF, CR, vertical tab and form feed.
const asciiWhitespace = " \t\n\r\v\f"

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
func JSONRecords(lines iter.Seq2[int, []byte], problems *Problems) iter.Seq2[int, pyjson.Value] {
	return func(yield func(int, pyjson.Value) bool) {
		for number, raw := range lines {
			stripped := bytes.Trim(raw, asciiWhitespace)
			if len(stripped) == 0 {
				continue
			}
			record, err := pyjson.Decode(stripped)
			if err == nil && !record.IsObject() {
				err = errNotObject
			}
			if err != nil {
				problems.AddRaw(number, UnparseableLine(err), LineText(raw))
				continue
			}
			if !yield(number, record) {
				return
			}
		}
	}
}

// JSONLines yields (line number, record) for each JSON object line of the
// file at path: the lines of Lines read as JSONRecords reads them.
func JSONLines(path string, problems *Problems) iter.Seq2[int, pyjson.Value] {
	return JSONRecords(Lines(path, problems), problems)
}

// LineText returns a raw line as the text of an error row: without its
// trailing LF and then CR, nothing else, decoded as UTF-8 with replacement.
func LineText(raw []byte) string {
	raw = bytes.TrimSuffix(raw, []byte("\n"))
	raw = bytes.TrimSuffix(raw, []byte("\r"))
	return pytext.DecodeUTF8(raw)
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
