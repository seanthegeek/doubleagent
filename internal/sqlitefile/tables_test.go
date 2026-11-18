package sqlitefile

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// tableDump is one table of a dump: its column names, rows and the error
// SQLite met reading it.
type tableDump struct {
	name    string
	columns []string
	rows    []string
	err     string
}

// siteDump is one query's result in a dump.
type siteDump struct {
	rows []string
	err  string
}

// databaseDump is one `db` section of tables.txt or of the fixture dump.
type databaseDump struct {
	name   string
	tables []*tableDump
	sites  map[string]*siteDump
	err    string
}

// loadDump parses a dump of tables and query results: testdata/tables.txt
// or testdata/fixture_tables.txt.
func loadDump(t *testing.T, path string) []*databaseDump {
	t.Helper()
	data, err := osfile.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var databases []*databaseDump
	var table *tableDump
	var site *siteDump
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(nil, 1<<24)
	for scanner.Scan() {
		word, rest, _ := strings.Cut(scanner.Text(), " ")
		if word == "db" {
			databases = append(databases, &databaseDump{name: rest, sites: map[string]*siteDump{}})
			table, site = nil, nil
			continue
		}
		if len(databases) == 0 {
			continue
		}
		current := databases[len(databases)-1]
		switch word {
		case "table":
			name, _ := hex.DecodeString(rest)
			table, site = &tableDump{name: string(name)}, nil
			current.tables = append(current.tables, table)
		case "columns":
			table.columns = decodeNames(rest)
		case "site":
			site, table = &siteDump{}, nil
			current.sites[rest] = site
		case "row", "error":
			if table == nil && site == nil {
				current.err = rest
			}
			addDumpLine(table, site, word, rest)
		}
	}
	return databases
}

// decodeNames decodes a `columns` line's hex names.
func decodeNames(text string) []string {
	var names []string
	for _, field := range strings.Fields(text) {
		name, _ := hex.DecodeString(field)
		names = append(names, string(name))
	}
	return names
}

// addDumpLine adds a row or an error to the table or query being read.
func addDumpLine(table *tableDump, site *siteDump, word, rest string) {
	switch {
	case table != nil && word == "row":
		table.rows = append(table.rows, rest)
	case table != nil:
		table.err = rest
	case site != nil && word == "row":
		site.rows = append(site.rows, rest)
	case site != nil:
		site.err = rest
	}
}

// describeRow writes a row as the dumps do: the rowid, or - without one,
// then each value.
func describeRow(rowid string, values []Value) string {
	words := []string{rowid}
	for _, value := range values {
		words = append(words, describeValue(value))
	}
	return strings.Join(words, " ")
}

// dumpTable reads a whole table as the dump does and returns its columns
// and rows.
func dumpTable(database *Database, name string) ([]string, []string, error) {
	table, err := database.Table(name)
	if err != nil {
		return nil, nil, err
	}
	rows, err := table.Select().All()
	if err != nil {
		return nil, nil, err
	}
	var columns []string
	for _, column := range table.Columns() {
		columns = append(columns, column.Name)
	}
	var lines []string
	for _, row := range rows {
		rowid := strconv.FormatInt(row.Rowid, 10)
		if table.WithoutRowid() {
			rowid = "-"
		}
		lines = append(lines, describeRow(rowid, row.Values()))
	}
	return columns, lines, nil
}

// unreadable lists the tables this package refuses although SQLite reads
// them, with the reason, so a test fails on any other refusal.
func unreadable() map[string]string {
	return map[string]string{
		"computed": "a virtual generated column is computed by SQL when read",
	}
}

// compareTables checks every table of a dumped database against the reader.
func compareTables(t *testing.T, database *Database, expected *databaseDump) {
	for _, table := range expected.tables {
		columns, rows, err := dumpTable(database, table.name)
		switch {
		case unreadable()[table.name] != "":
			if err == nil {
				t.Errorf("table %s read, but this package should refuse it: %s", table.name,
					unreadable()[table.name])
			}
		case table.err != "":
			if err == nil {
				t.Errorf("table %s read without error; SQLite reports %q", table.name, table.err)
			}
		case err != nil:
			t.Errorf("table %s: %v", table.name, err)
		case !slices.Equal(columns, table.columns):
			t.Errorf("table %s columns = %q, want %q", table.name, columns, table.columns)
		case !slices.Equal(rows, table.rows):
			t.Errorf("table %s rows:\n%s", table.name, lineDifference(rows, table.rows))
		}
	}
}

// compareSites runs every rewritten query against a dumped database.
func compareSites(t *testing.T, database *Database, expected *databaseDump) {
	for name, site := range sites() {
		rows, applicable, err := site(database)
		want, dumped := expected.sites[name]
		switch {
		case applicable != dumped:
			t.Errorf("query %s applies = %v, the dump records it = %v (error %v)", name, applicable,
				dumped, err)
		case !dumped:
		case want.err != "":
			if err == nil {
				t.Errorf("query %s ran without error; SQLite reports %q", name, want.err)
			}
		case err != nil:
			t.Errorf("query %s: %v", name, err)
		default:
			var lines []string
			for _, row := range rows {
				lines = append(lines, strings.TrimPrefix(describeRow("", row), " "))
			}
			if !slices.Equal(lines, want.rows) {
				t.Errorf("query %s rows:\n%s", name, lineDifference(lines, want.rows))
			}
		}
	}
}

// lineDifference shows two line lists side by side from their first difference.
func lineDifference(got, want []string) string {
	first := firstDifference(got, want)
	var out strings.Builder
	for i := first; i < max(len(got), len(want)) && i < first+6; i++ {
		gotLine, wantLine := "<none>", "<none>"
		if i < len(got) {
			gotLine = got[i]
		}
		if i < len(want) {
			wantLine = want[i]
		}
		fmt.Fprintf(&out, "  %d got  %s\n  %d want %s\n", i, gotLine, i, wantLine)
	}
	return out.String()
}

func TestTablesAndQueriesReadAsSQLiteReadsThem(t *testing.T) {
	databases := loadDump(t, filepath.Join("testdata", "tables.txt"))
	if len(databases) < 3 {
		t.Fatalf("tables.txt holds %d databases, fewer than testdata holds", len(databases))
	}
	for _, expected := range databases {
		t.Run(expected.name+" reads as SQLite reads it", func(t *testing.T) {
			database, err := OpenPath(filepath.Join("testdata", expected.name))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			compareTables(t, database, expected)
			compareSites(t, database, expected)
		})
	}
}

func TestInvalidTextIsAnErrorNamingTheRow(t *testing.T) {
	database := openTestdata(t, "schema.db")
	table, err := database.Table("broken")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := table.Select("a", "b").All()
	if !errors.Is(err, ErrInvalidText) || !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("All() error = %v, want ErrInvalidText naming row 2", err)
	}
	if len(rows) != 1 || rows[0].Get("a").Text() != "ok" {
		t.Errorf("rows before the bad one = %d, want the one before it", len(rows))
	}
	if rows, err := table.Select("b").All(); err != nil || len(rows) != 3 {
		t.Errorf("a selection without the column read %d rows, error %v", len(rows), err)
	}
}

// openTestdata opens a test database and closes it when the test ends.
func openTestdata(t *testing.T, name string) *Database {
	t.Helper()
	database, err := OpenPath(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
