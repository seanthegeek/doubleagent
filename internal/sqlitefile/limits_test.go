package sqlitefile

import (
	"encoding/binary"
	"errors"
	"fmt"
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

// fatTable returns a table declared with three columns whose count rows,
// one to a page, each store fields values: three integers, then NULLs.
func fatTable(t *testing.T, count, fields int) *Table {
	t.Helper()
	table, err := openOrFail(t, fatTableFile(count, fields), nil, nil).Table("fat")
	if err != nil {
		t.Fatal(err)
	}
	return table
}

// fatTableFile returns the database file of fatTable.
func fatTableFile(count, fields int) []byte {
	pages := newTestPages(maxPageSize)
	var leaves []uint32
	for i := range count {
		values := make([]Value, fields)
		values[0], values[1], values[2] = IntValue(int64(i)), IntValue(int64(count-i)), IntValue(7)
		leaf := pages.add()
		pages.setPage(leaf, leafTable, [][]byte{tableLeafCell(int64(i+1), testRecord(values...))}, 0)
		leaves = append(leaves, leaf)
	}
	root := pages.add()
	var dividers [][]byte
	for i, leaf := range leaves[:len(leaves)-1] {
		cell := binary.BigEndian.AppendUint32(nil, leaf)
		dividers = append(dividers, appendSQLiteVarint(cell, uint64(i+1)))
	}
	pages.setPage(root, interiorTable, dividers, leaves[len(leaves)-1])
	pages.setSchema(schemaTable{"fat", root, "CREATE TABLE fat(a, b, c)"})
	return pages.bytes()
}

// fatReads are the ways of reading a table, each returning the rows it read.
func fatReads() []struct {
	name string
	read func(table *Table) ([]Row, error)
} {
	return []struct {
		name string
		read func(table *Table) ([]Row, error)
	}{
		{"every column", func(table *Table) ([]Row, error) { return table.Select().All() }},
		{"one selected column", func(table *Table) ([]Row, error) { return table.Select("a").All() }},
		{"a sorted query", func(table *Table) ([]Row, error) {
			return table.Select("a").OrderBy("b DESC").All()
		}},
		{"a filtered query", func(table *Table) ([]Row, error) {
			return table.Select("a").Where(func(row Row) bool { return row.Get("c").Int == 7 }).All()
		}},
		{"a lookup", func(table *Table) ([]Row, error) {
			lookup, err := table.Select("a").Lookup("b")
			if err != nil {
				return nil, err
			}
			return lookup.rows, nil
		}},
		{"the values of whole rows", func(table *Table) ([]Row, error) {
			rows, err := table.Select().All()
			for _, row := range rows {
				if values := row.Values(); len(values) != 3 || values[2].Int != 7 {
					return nil, fmt.Errorf("Values() = %d values, want a, b and c", len(values))
				}
			}
			return rows, err
		}},
	}
}

func TestAWideRecordCostsOnlyTheDeclaredColumns(t *testing.T) {
	// Fields past the last declared column are walked for the exact-fill
	// check but never decoded: a row of 16,000 stored fields costs its page,
	// not 48 bytes a field.
	const fields, perRow = 16000, 2 * maxPageSize
	for _, test := range fatReads() {
		t.Run(test.name+" keeps the declared columns of each record", func(t *testing.T) {
			rows, err := test.read(fatTable(t, 4, fields))
			if err != nil || len(rows) != 4 {
				t.Fatalf("read = %d rows, %v; want 4", len(rows), err)
			}
			for _, row := range rows {
				if len(row.values) > 3 {
					t.Errorf("row %d holds %d values, want at most the 3 declared", row.Rowid, len(row.values))
				}
			}
		})
		t.Run(test.name+" costs a page per row however many fields a record stores", func(t *testing.T) {
			cost := map[int]uint64{}
			for _, count := range []int{8, 16} {
				table := fatTable(t, count, fields)
				cost[count] = allocatedBytes(func() {
					if rows, err := test.read(table); err != nil || len(rows) != count {
						t.Fatalf("read = %d rows, %v; want %d", len(rows), err, count)
					}
				})
			}
			if grown := cost[16] - cost[8]; grown > 8*perRow {
				t.Errorf("8 more rows allocated %d bytes, more than %d per row", grown, perRow)
			}
		})
	}
}

func TestAWhereRowHoldsOnlyTheDeclaredColumns(t *testing.T) {
	seen := 0
	_, err := fatTable(t, 2, 16000).Select("a").Where(func(row Row) bool {
		if len(row.values) > 3 {
			t.Errorf("the row given to Where holds %d values, want at most 3", len(row.values))
		}
		seen++
		return true
	}).OrderBy("b").All()
	if err != nil || seen != 2 {
		t.Errorf("Where saw %d rows, %v; want 2", seen, err)
	}
}
