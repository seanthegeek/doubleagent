package codex

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/zstdframe"
)

// numberedRecord is one JSON object line of a file and its line number.
type numberedRecord struct {
	line  int
	value jsonvalue.Value
}

// asciiWhitespace is what Python's bytes.strip() removes from a line.
const asciiWhitespace = " \t\n\r\v\f"

// zstRecords returns the JSON object lines of a zstd-compressed JSON lines
// file, numbered from 1, as zstRecordsOf reads them. A file that cannot be
// opened adds its unreadable problem at line 0.
func zstRecords(path string, problems *parser.Problems) []numberedRecord {
	file, err := parser.Open(path)
	if err != nil {
		problems.AddUnreadable(0, path, err)
		return nil
	}
	defer file.Close()
	return zstRecordsOf(file, path, problems)
}

// zstRecordsOf decodes the zstd frames of r, one after another, and returns
// the JSON object lines of the text they hold. A line that is not JSON adds
// an "unparseable line" problem and one holding another JSON value is
// skipped. Corrupt or cut data adds one "zstd:" problem at the line after
// the last one decoded, after which the text decoded before the damage is
// read as a last line. A read that fails adds the file's unreadable problem
// at the last line read, and nothing more is read; the file is read in
// chunks of readChunk bytes, so the lines of the chunks before the failing
// one are kept.
func zstRecordsOf(r io.Reader, path string, problems *parser.Problems) []numberedRecord {
	var records []numberedRecord
	var pending []byte
	number := 0
	source := &chunkReader{source: r, buffer: make([]byte, readChunk)}
	err := zstdframe.DecodeStream(source, func(chunk []byte) error {
		pending = append(pending, chunk...)
		for {
			end := bytes.IndexByte(pending, '\n')
			if end < 0 {
				break
			}
			number++
			records = appendRecord(records, number, pending[:end], problems)
			pending = pending[end+1:]
		}
		return nil
	})
	if err != nil {
		if source.err != nil && !errors.Is(source.err, io.EOF) {
			problems.AddUnreadable(number, path, source.err)
			return records
		}
		problems.Add(number+1, zstdMessage(err))
	}
	if len(bytes.Trim(pending, asciiWhitespace)) > 0 {
		records = appendRecord(records, number+1, pending, problems)
	}
	return records
}

// appendRecord adds the line raw to records when it holds a JSON object; a
// blank line adds nothing, and a line that is not JSON adds its problem.
func appendRecord(records []numberedRecord, number int, raw []byte,
	problems *parser.Problems) []numberedRecord {
	stripped := bytes.Trim(raw, asciiWhitespace)
	if len(stripped) == 0 {
		return records
	}
	record, err := jsonvalue.Decode(stripped)
	if err != nil {
		problems.AddRaw(number, parser.UnparseableLine(err), parser.LineText(raw))
		return records
	}
	if !record.IsObject() {
		return records
	}
	return append(records, numberedRecord{line: number, value: record})
}

// zstdMessage returns the "zstd:" problem message of a decoding failure.
func zstdMessage(err error) string {
	switch {
	case errors.Is(err, zstdframe.ErrTruncated):
		return "zstd: the file ends inside a zstd frame"
	case errors.Is(err, zstdframe.ErrTooLarge):
		return "zstd: a frame needs a window larger than the decoder allows"
	}
	detail := strings.TrimPrefix(err.Error(), zstdframe.ErrCorrupt.Error()+": ")
	return "zstd: corrupt data (" + detail + ")"
}

// readChunk is how much of a compressed file is read at a time.
const readChunk = 64 << 10

// chunkReader reads its source a chunk at a time, one read per chunk, and
// hands the decoder the chunk's bytes in whatever sizes it asks for, so the
// decoder's small reads cost no system call each and a read that fails is
// met only once the chunks before it are used up. It keeps the source's
// error.
type chunkReader struct {
	source     io.Reader
	buffer     []byte
	start, end int
	err        error
}

// Read gives the next bytes of the current chunk, reading the next chunk
// when it is used up.
func (c *chunkReader) Read(out []byte) (int, error) {
	if c.start == c.end {
		if c.err != nil {
			return 0, c.err
		}
		n, err := c.source.Read(c.buffer)
		c.start, c.end, c.err = 0, n, err
		if n == 0 {
			return 0, err
		}
	}
	n := copy(out, c.buffer[c.start:c.end])
	c.start += n
	return n, nil
}
