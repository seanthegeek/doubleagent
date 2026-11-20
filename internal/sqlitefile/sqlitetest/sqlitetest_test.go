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
