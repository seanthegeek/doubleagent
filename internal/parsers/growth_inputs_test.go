package parsers

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The builders the growth cases write their files with.

// longValue is the length of the one long value of a case of
// shapeCarried or shapeLargeShared: longer than rowAllowance, so that a
// parse writing it whole again per record breaks the bound, while one
// that cuts it to parser.CarriedLimit passes.
const longValue = 50_000

// long returns a value of longValue bytes that starts with name, so two
// long values of one case differ.
func long(name string) string {
	return name + strings.Repeat("v", longValue-len(name))
}

// numbered returns prefix followed by i.
func numbered(prefix string, i int) string {
	return prefix + strconv.Itoa(i)
}

// lines returns n lines, line i being line(i), each ended by a line feed.
func lines(n int, line func(i int) string) []byte {
	var out strings.Builder
	for i := range n {
		out.WriteString(line(i))
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

// joined returns n items joined by commas, item i being item(i): the
// members of a JSON array or object.
func joined(n int, item func(i int) string) string {
	items := make([]string, 0, n)
	for i := range n {
		items = append(items, item(i))
	}
	return strings.Join(items, ",")
}

// nestedArrays returns depth JSON arrays, each inside the one before,
// around inner.
func nestedArrays(depth int, inner string) string {
	return strings.Repeat("[", depth) + inner + strings.Repeat("]", depth)
}

// nestedObjects returns depth JSON objects, each the value of the member
// key of the one before, around inner.
func nestedObjects(depth int, key, inner string) string {
	return strings.Repeat(`{"`+key+`":`, depth) + inner + strings.Repeat("}", depth)
}

// one returns a single file.
func one(rel string, data []byte) []growthFile {
	return []growthFile{{rel: rel, data: data}}
}

// text returns a file holding text.
func text(rel, body string) growthFile {
	return growthFile{rel: rel, data: []byte(body)}
}

// database returns the bytes of a SQLite database holding the tables,
// failing the test when they cannot be written.
func database(t *testing.T, tables ...sqlitetest.Table) []byte {
	t.Helper()
	data, err := sqlitetest.Build(tables...)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// rows returns n rows, row i being row(i).
func rows(n int, row func(i int) []any) [][]any {
	all := make([][]any, 0, n)
	for i := range n {
		all = append(all, row(i))
	}
	return all
}

// manyFiles returns n files, file i at rel(i) holding body(i).
func manyFiles(n int, rel func(i int) string, body func(i int) string) []growthFile {
	files := make([]growthFile, 0, n)
	for i := range n {
		files = append(files, text(rel(i), body(i)))
	}
	return files
}

// zstdBlock is the most bytes one raw block of a zstd frame holds (RFC 8878,
// section 3.1.1.2: the block maximum, 128 KiB).
const zstdBlock = 128 << 10

// zstdFrame returns data as one zstd frame of raw, uncompressed blocks
// (RFC 8878, section 3.1.1): the magic number, a frame header holding only
// a window of 2^windowLog bytes (from 2^10 up), then each block after its
// three-byte header: its size times eight, plus one on the last block. A
// window of 2^27, the largest the decoder accepts, makes a parse that
// reserves a window per frame allocate that much per frame.
func zstdFrame(windowLog byte, data []byte) []byte {
	frame := []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, (windowLog - 10) << 3}
	for start := 0; start == 0 || start < len(data); start += zstdBlock {
		block := data[start:min(len(data), start+zstdBlock)]
		header := len(block) << 3
		if start+zstdBlock >= len(data) {
			header |= 1
		}
		frame = append(append(frame, byte(header), byte(header>>8), byte(header>>16)), block...)
	}
	return frame
}
