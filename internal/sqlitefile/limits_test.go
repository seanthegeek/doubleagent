package sqlitefile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
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
				OrderBy("c1")
		}},
	}
	for _, test := range queries {
		t.Run(test.name+" costs the same per row whatever the width", func(t *testing.T) {
			cost := map[int]int64{}
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
			return "((" + strings.Repeat("- ", depth-2) + "7))"
		}, 7},
		{"CASTs, each a word and a parenthesis", func(depth int) string {
			casts := (depth - 1) / 2
			return "(" + strings.Repeat("CAST(", casts) + "7" + strings.Repeat(" AS INTEGER)", casts) + ")"
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
				!strings.Contains(shape.columns[0].defaultErr.Error(), "more than "+strconv.Itoa(maxDefaultDepth)+" deep") {
				t.Errorf("default error = %v (%v), want one naming the limit", shape.columns[0].defaultErr, err)
			}
		})
	}
}

// Each level of a DEFAULT is counted as it is scanned, so a value nested
// past the limit is refused before the tokens beyond it are kept, and the
// memory it takes does not grow with its length.
func TestADefaultNestedPastTheLimitKeepsNoTokensPastIt(t *testing.T) {
	tests := []struct {
		name   string
		nested func(count int) string
	}{
		{"parentheses", func(count int) string { return strings.Repeat("(", count) + "7" + strings.Repeat(")", count) }},
		{"signs", func(count int) string { return "(" + strings.Repeat("- ", count) + "7)" }},
		{"CASTs", func(count int) string {
			return "(" + strings.Repeat("CAST(", count) + "7" + strings.Repeat(" AS INTEGER)", count) + ")"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, count := range []int{100_000, 400_000} {
				sql := "CREATE TABLE t(a DEFAULT " + test.nested(count) + ")"
				var shape *tableShape
				var err error
				allocated := allocatedBytes(func() { shape, err = parseCreateTable(sql) })
				t.Logf("a %d-byte statement allocated %d bytes", len(sql), allocated)
				if err != nil || !errors.Is(shape.columns[0].defaultErr, errTooDeep) || allocated > 1<<20 {
					t.Errorf("a %d-byte statement allocated %d bytes and gave %v, %v; want at most %d bytes and the "+
						"depth refused", len(sql), allocated, err, shape.columns[0].defaultErr, 1<<20)
				}
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
			return table.Select("a").OrderBy("b").All()
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
			cost := map[int]int64{}
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

// manyTables returns a database of count tables, t0 to t<count-1>, each
// of one column holding no rows, and a table named Messages holding one
// row; reads through it add to read.
func manyTables(t *testing.T, count int, read *int64) *Database {
	t.Helper()
	pages := newTestPages(maxPageSize)
	tables := make([]schemaTable, 0, count+1)
	for i := range count {
		root := pages.add()
		pages.setPage(root, leafTable, nil, 0)
		name := "t" + strconv.Itoa(i)
		tables = append(tables, schemaTable{name, root, "CREATE TABLE " + name + "(a)"})
	}
	root := pages.add()
	pages.setPage(root, leafTable, [][]byte{tableLeafCell(1, testRecord(TextValue("kept")))}, 0)
	tables = append(tables, schemaTable{"Messages", root, "CREATE TABLE Messages(body)"})
	pages.setSchema(tables...)
	database, err := Open(countedFiles(read, pages.bytes(), nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	return database
}

func TestTheSchemaIsReadOncePerDatabase(t *testing.T) {
	var read int64
	database := manyTables(t, 200, &read)
	if _, err := database.Table("t0"); err != nil {
		t.Fatal(err)
	}
	once := read
	for i := range 200 {
		if _, err := database.Table("t" + strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.HasTable("absent"); err != nil {
		t.Fatal(err)
	}
	if read != once {
		t.Errorf("200 more table lookups read %d bytes, want none after the first read %d", read-once, once)
	}
}

func TestHasTableMatchesANameAsSQLDoes(t *testing.T) {
	var read int64
	database := manyTables(t, 2, &read)
	cases := []struct {
		name string
		want bool
	}{
		{"Messages", true},
		{"messages", true},
		{"MESSAGES", true},
		{"me\u017fsages", false},
		{"t1", true},
		{"absent", false},
	}
	for _, test := range cases {
		t.Run("a table called "+test.name+" is found as SQL finds it", func(t *testing.T) {
			if got, err := database.HasTable(test.name); err != nil || got != test.want {
				t.Errorf("HasTable(%s) = %v, %v; want %v", test.name, got, err, test.want)
			}
			_, err := database.Table(test.name)
			if found := err == nil; found != test.want {
				t.Errorf("Table(%s) error = %v, want found %v as HasTable says", test.name, err, test.want)
			}
		})
	}
}

func TestATableDeclaringAColumnTwiceLeavesTheOthersReadable(t *testing.T) {
	pages := newTestPages(4096)
	twice, kept := pages.add(), pages.add()
	pages.setPage(twice, leafTable, [][]byte{tableLeafCell(1, testRecord(IntValue(1), IntValue(2)))}, 0)
	pages.setPage(kept, leafTable, [][]byte{tableLeafCell(1, testRecord(TextValue("kept")))}, 0)
	pages.setSchema(schemaTable{"twice", twice, "CREATE TABLE twice(id integer primary key, a, a)"},
		schemaTable{"kept", kept, "CREATE TABLE kept(a)"})
	database := openOrFail(t, pages.bytes(), nil, nil)
	if _, err := database.Table("twice"); !errors.Is(err, errSchemaSyntax) ||
		!strings.Contains(err.Error(), "duplicate column name: a") {
		t.Errorf("Table(twice) error = %v, want the duplicate column refused", err)
	}
	table, err := database.Table("kept")
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := table.Select("a").All(); err != nil || len(rows) != 1 || rows[0].Get("a").Text() != "kept" {
		t.Errorf("Select(a) = %v, %v; want the one row", rows, err)
	}
}

func TestTablesMayBeLookedUpFromSeveralGoroutines(t *testing.T) {
	var read int64
	database := manyTables(t, 20, &read)
	var group sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		group.Go(func() {
			for i := range 20 {
				if _, err := database.Table("t" + strconv.Itoa(i)); err != nil {
					failures <- err
					return
				}
			}
		})
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func TestARefusedStatementHoldsNoTokensPastTheRefusal(t *testing.T) {
	tests := []struct {
		name, want string
		sql        func(count int) string
		bound      int64
	}{
		{"a parenthesis left open", "a parenthesis is not closed", func(count int) string {
			return "CREATE TABLE t(a, b DEFAULT " + strings.Repeat("(", count)
		}, 64 << 10},
		{"columns past the limit", "more than 2000 columns", func(count int) string {
			return "CREATE TABLE t(" + columnList(count) + ")"
		}, 4 << 20},
		{"a long DEFAULT before a string left open", "unterminated", func(count int) string {
			return "CREATE TABLE t(a DEFAULT (" + strings.Repeat("1+", count) + "1), b 'x)"
		}, 64 << 10},
		{"a PRIMARY KEY past the limit", "PRIMARY KEY lists more than 2000 columns", func(count int) string {
			return "CREATE TABLE t(a, PRIMARY KEY (" + strings.Repeat("a, ", count) + "a))"
		}, 64 << 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, count := range []int{100_000, 400_000} {
				sql := test.sql(count)
				var err error
				allocated := allocatedBytes(func() { _, err = parseCreateTable(sql) })
				t.Logf("a %d-byte statement allocated %d bytes", len(sql), allocated)
				if err == nil || !strings.Contains(err.Error(), test.want) || allocated > test.bound {
					t.Errorf("a %d-byte statement allocated %d bytes and gave %v; want at most %d bytes and %q",
						len(sql), allocated, err, test.bound, test.want)
				}
			}
		})
	}
}
