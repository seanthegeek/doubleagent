package sqlitefile

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// columnList returns the names c0 to c<count-1> joined by commas.
func columnList(count int) string {
	names := make([]string, count)
	for i := range names {
		names[i] = "c" + strconv.Itoa(i)
	}
	return strings.Join(names, ", ")
}

func TestColumnListsAreLimitedAsSQLiteLimitsThem(t *testing.T) {
	cases := []struct {
		name string
		sql  func(count int) string
	}{
		{"a table's columns", func(count int) string {
			return "CREATE TABLE t(" + columnList(count) + ")"
		}},
		{"an fts5 table's columns", func(count int) string {
			return "CREATE VIRTUAL TABLE f USING fts5(" + columnList(count) + ")"
		}},
		{"a PRIMARY KEY's columns", func(count int) string {
			return "CREATE TABLE t(a, b, PRIMARY KEY(" + strings.Repeat("a, ", count-1) + "a))"
		}},
	}
	for _, test := range cases {
		t.Run(test.name+" may number up to the limit", func(t *testing.T) {
			if _, err := parseCreateTable(test.sql(maxColumns)); err != nil {
				t.Errorf("parseCreateTable() of %d = %v, want no error", maxColumns, err)
			}
		})
		t.Run(test.name+" are refused past the limit", func(t *testing.T) {
			_, err := parseCreateTable(test.sql(maxColumns + 1))
			if !errors.Is(err, errSchemaSyntax) || !strings.Contains(err.Error(), "more than 2000 columns") {
				t.Errorf("parseCreateTable() of %d = %v, want a refusal naming the limit", maxColumns+1, err)
			}
		})
	}
}

func TestAWideTableLeavesTheOtherTablesReadable(t *testing.T) {
	pages := newTestPages(maxPageSize)
	wide, small := pages.add(), pages.add()
	pages.setPage(wide, leafTable, nil, 0)
	pages.setPage(small, leafTable, [][]byte{tableLeafCell(1, testRecord(TextValue("kept")))}, 0)
	pages.setSchema(schemaTable{"wide", wide, "CREATE TABLE wide(" + columnList(maxColumns+1) + ")"},
		schemaTable{"small", small, "CREATE TABLE small(a)"})
	database := openOrFail(t, pages.bytes(), nil, nil)
	if _, err := database.Table("wide"); !errors.Is(err, errSchemaSyntax) {
		t.Errorf("Table(wide) error = %v, want the CREATE TABLE refused", err)
	}
	table, err := database.Table("small")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := table.Select("a").All()
	if err != nil || len(rows) != 1 || rows[0].Get("a").Text() != "kept" {
		t.Errorf("Select(a) = %v, %v; want the one row", rows, err)
	}
}

// wideTable returns a table of maxColumns columns, the last with a
// default, holding count rows whose records store only the first column.
func wideTable(t *testing.T, count int) *Table {
	t.Helper()
	pages := newTestPages(maxPageSize)
	root := pages.add()
	cells := make([][]byte, count)
	for i := range cells {
		cells[i] = tableLeafCell(int64(i+1), testRecord(IntValue(int64(i))))
	}
	pages.setPage(root, leafTable, cells, 0)
	sql := "CREATE TABLE wide(" + columnList(maxColumns-1) + ", last DEFAULT 7)"
	pages.setSchema(schemaTable{"wide", root, sql})
	table, err := openOrFail(t, pages.bytes(), nil, nil).Table("wide")
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func TestTheCostOfARowFollowsItsRecordNotTheDeclaredColumns(t *testing.T) {
	// A row stored as one value costs a few hundred bytes to read, however
	// many columns the table declares; holding the declared columns would
	// cost 48 bytes each.
	const perRow = 1024
	queries := []struct {
		name  string
		query func(table *Table) Query
	}{
		{"one selected column", func(table *Table) Query { return table.Select("c0") }},
		{"every column", func(table *Table) Query { return table.Select() }},
		{"a filtered and sorted query", func(table *Table) Query {
			return table.Select("c0").Where(func(row Row) bool { return row.Get("last").Int == 7 }).
				OrderBy("c1 DESC")
		}},
	}
	for _, test := range queries {
		t.Run(test.name+" costs the same per row whatever the width", func(t *testing.T) {
			cost := map[int]uint64{}
			for _, count := range []int{500, 1000} {
				table := wideTable(t, count)
				cost[count] = allocatedBytes(func() {
					rows, err := test.query(table).All()
					if err != nil || len(rows) != count {
						t.Fatalf("All() = %d rows, %v; want %d", len(rows), err, count)
					}
					if last := rows[len(rows)-1]; !last.Has("c0") || last.Get("c0").Type != Integer {
						t.Fatalf("last row = %v, want its stored c0", last.Values())
					}
				})
			}
			if grown := cost[1000] - cost[500]; grown > 500*perRow {
				t.Errorf("500 more rows allocated %d bytes, more than %d per row", grown, perRow)
			}
		})
	}
}

func TestShortRecordsReadTheDeclaredDefaults(t *testing.T) {
	rows, err := wideTable(t, 3).Select().All()
	if err != nil || len(rows) != 3 {
		t.Fatalf("Select().All() = %d rows, %v", len(rows), err)
	}
	values := rows[2].Values()
	if len(values) != maxColumns || values[0].Int != 2 || values[1].Type != Null ||
		values[maxColumns-1].Int != 7 {
		t.Errorf("row 3 = %d values starting %v and ending %v; want 2, NULL, ..., 7",
			len(values), values[:2], values[len(values)-1])
	}
}

func TestDefaultsNestedPastSQLitesParserAreNotEvaluated(t *testing.T) {
	cases := []struct {
		name    string
		nesting func(depth int) string
		want    int64
	}{
		{"parentheses", func(depth int) string {
			return strings.Repeat("(", depth) + "7" + strings.Repeat(")", depth)
		}, 7},
		{"signs in parentheses", func(depth int) string {
			return "(" + strings.Repeat("- ", depth) + "7)"
		}, 7},
	}
	for _, test := range cases {
		t.Run(test.name+" up to the limit are evaluated", func(t *testing.T) {
			shape, err := parseCreateTable("CREATE TABLE t(a DEFAULT " + test.nesting(maxDefaultDepth) + ")")
			if err != nil || shape.columns[0].defaultErr != nil || shape.columns[0].Default.Int != test.want {
				t.Errorf("default = %v, %v (%v); want %d", shape.columns[0].Default,
					shape.columns[0].defaultErr, err, test.want)
			}
		})
		t.Run(test.name+" past the limit are refused", func(t *testing.T) {
			shape, err := parseCreateTable("CREATE TABLE t(a DEFAULT " + test.nesting(maxDefaultDepth+2) + ")")
			if err != nil || !errors.Is(shape.columns[0].defaultErr, errDefault) ||
				!strings.Contains(shape.columns[0].defaultErr.Error(), "more than 100 deep") {
				t.Errorf("default error = %v (%v), want one naming the limit", shape.columns[0].defaultErr, err)
			}
		})
	}
}
