package sqlitetest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// A database written by Build reads back through sqlitefile with every
// row in rowid order and every value as written, whether the table fits
// on one leaf page, needs interior pages above many leaves, or holds text
// longer than a page that spills onto overflow pages.
func TestBuildWritesTablesTheReaderReadsBack(t *testing.T) {
	tests := []struct {
		name       string
		rows, size int
	}{
		{"one leaf page", 3, 10},
		{"many leaves under one interior page", 400, 100},
		{"two levels of interior pages", 120_000, 1},
		{"text spilling onto overflow pages", 30, 20_000},
		{"text filling whole overflow pages", 2, 4096 - 4 + 489},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rows [][]any
			for i := range test.rows {
				rows = append(rows, []any{int64(i), strings.Repeat("x", test.size) + strconv.Itoa(i), nil, 1.5})
			}
			data, err := Build(Table{Name: "items", SQL: "CREATE TABLE items (id INTEGER, body TEXT, gone, ratio REAL)",
				Rows: rows}, Table{Name: "empty", SQL: "CREATE TABLE empty (id INTEGER)"})
			if err != nil {
				t.Fatal(err)
			}
			database, err := sqlitefile.OpenBytes(data, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			table, err := database.Table("items")
			if err != nil {
				t.Fatal(err)
			}
			read, err := table.Select("id", "body", "gone", "ratio").All()
			if err != nil {
				t.Fatal(err)
			}
			if len(read) != test.rows {
				t.Fatalf("read %d rows, want %d", len(read), test.rows)
			}
			for i, row := range read {
				body := strings.Repeat("x", test.size) + strconv.Itoa(i)
				if row.Get("id").Int != int64(i) || row.Get("body").Text() != body ||
					row.Get("gone").Type != sqlitefile.Null || row.Get("ratio").Float != 1.5 {
					t.Fatalf("row %d reads back as %v", i, row.Values())
				}
			}
			empty, err := database.Table("empty")
			if err != nil {
				t.Fatal(err)
			}
			if got, err := empty.Select("id").All(); err != nil || len(got) != 0 {
				t.Errorf("the empty table gave %d rows and %v", len(got), err)
			}
		})
	}
}

// A value of a type the file format has no serial type for is an error.
func TestBuildRefusesAValueOfAnotherType(t *testing.T) {
	if _, err := Build(Table{Name: "t", SQL: "CREATE TABLE t (a)", Rows: [][]any{{true}}}); err == nil {
		t.Error("a boolean value was written")
	}
}

// readVarint returns the varint at the start of data and its length, for
// a value of at most 56 bits.
func readVarint(data []byte) (uint64, int) {
	var value uint64
	for i, part := range data {
		value = value<<7 | uint64(part&0x7f)
		if part&0x80 == 0 {
			return value, i + 1
		}
	}
	return value, len(data)
}

// A record's header size counts the varint that holds it, however many
// bytes that takes: one, two, or three past a header of 16,382 serial
// types, where adding the size's own bytes needs one more.
func TestARecordsHeaderSizeCountsItsOwnVarint(t *testing.T) {
	for _, columns := range []int{1, 126, 127, 16381, 16382, 16383} {
		t.Run(strconv.Itoa(columns)+" columns", func(t *testing.T) {
			record, err := encodeRecord(make([]any, columns))
			if err != nil {
				t.Fatal(err)
			}
			size, length := readVarint(record)
			if int(size) != length+columns {
				t.Errorf("header size %d, want %d (a %d-byte size and %d serial types)", size, length+columns,
					length, columns)
			}
		})
	}
}

// A varint is written for a value of at most 56 bits, in eight bytes at
// most, and a larger value, which needs the format's nine-byte form and
// which no table of the package's rowids and sizes reaches, is refused.
func TestAVarintPast56BitsIsRefused(t *testing.T) {
	if got := appendVarint(nil, 1<<56-1); len(got) != 8 || got[7] != 0x7f {
		t.Errorf("2^56-1 is written % x", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("2^56 was written")
		}
	}()
	appendVarint(nil, 1<<56)
}
