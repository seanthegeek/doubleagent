package sqlitefile

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// The databases under testdata are built by testdata/generate.py, which
// also records in testdata/expected.txt what SQLite reads from each.

// expectedTree is one tree of expected.txt: its kind, its name in the
// schema, its rows in order, and the error SQLite met reading it.
type expectedTree struct {
	kind, name string
	rows       []string
	err        string
}

// expectedDatabase is one `db` section of expected.txt.
type expectedDatabase struct {
	name   string
	schema []string
	trees  []*expectedTree
	err    string
}

// loadExpected parses testdata/expected.txt.
func loadExpected(t *testing.T) []*expectedDatabase {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", "expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var databases []*expectedDatabase
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		word, rest, _ := strings.Cut(scanner.Text(), " ")
		if word == "db" {
			databases = append(databases, &expectedDatabase{name: rest})
			continue
		}
		if len(databases) == 0 {
			continue
		}
		current := databases[len(databases)-1]
		addExpected(current, word, rest)
	}
	return databases
}

// addExpected adds one line of a `db` section to it.
func addExpected(current *expectedDatabase, word, rest string) {
	var tree *expectedTree
	if len(current.trees) > 0 {
		tree = current.trees[len(current.trees)-1]
	}
	switch {
	case word == "schema":
		current.schema = append(current.schema, rest)
	case word == "tree":
		kind, name, _ := strings.Cut(rest, " ")
		current.trees = append(current.trees, &expectedTree{kind: kind, name: name})
	case word == "row":
		tree.rows = append(tree.rows, rest)
	case word == "error" && tree != nil:
		tree.err = rest
	case word == "error":
		current.err = rest
	}
}

// describeValue writes a value as expected.txt does.
func describeValue(value Value) string {
	switch value.Type {
	case Integer:
		return "int:" + strconv.FormatInt(value.Int, 10)
	case Float:
		bits := make([]byte, 8)
		binary.BigEndian.PutUint64(bits, math.Float64bits(value.Float))
		return "float:" + hex.EncodeToString(bits)
	case Text, Blob:
		kind := strings.ToLower(value.Type.String())
		if len(value.Bytes) <= 64 {
			return kind + ":" + hex.EncodeToString(value.Bytes)
		}
		sum := sha256.Sum256(value.Bytes)
		return fmt.Sprintf("%s:%d:%s", kind, len(value.Bytes), hex.EncodeToString(sum[:]))
	}
	return "null"
}

// describeSchema writes a schema row as expected.txt does.
func describeSchema(row schemaRecord) string {
	sql := hex.EncodeToString([]byte(row.SQL))
	if sql == "" {
		sql = "-"
	}
	return fmt.Sprintf("%s %s %s %d %s", row.Type, row.Name, row.TableName, row.RootPage, sql)
}

// readTree returns a tree's rows in expected.txt's form and the error that
// ended the walk.
func readTree(database *Database, kind string, root uint32) ([]string, error) {
	rows := database.indexRows(root)
	if kind == "table" {
		rows = database.tableRows(root)
	}
	var lines []string
	for rows.Next() {
		values, err := rows.Record()
		if err != nil {
			return lines, err
		}
		words := []string{"-"}
		if kind == "table" {
			words[0] = strconv.FormatInt(rows.Rowid(), 10)
		}
		for _, value := range values {
			words = append(words, describeValue(value))
		}
		lines = append(lines, strings.Join(words, " "))
	}
	return lines, rows.Err()
}

func TestReaderSeesWhatSQLiteSees(t *testing.T) {
	databases := loadExpected(t)
	if len(databases) < 40 {
		t.Fatalf("expected.txt holds %d databases, fewer than testdata holds", len(databases))
	}
	for _, expected := range databases {
		t.Run(expected.name+" reads as SQLite reads it", func(t *testing.T) {
			checkDatabase(t, expected)
		})
	}
}

// checkDatabase compares one database's schema and trees with expected.
func checkDatabase(t *testing.T, expected *expectedDatabase) {
	database, err := OpenPath(filepath.Join("testdata", expected.name), osfile.ListSpellings)
	if expected.name == "utf16le.db" {
		if !errors.Is(err, errUTF16) {
			t.Fatalf("OpenPath(, osfile.ListSpellings) error = %v, want errUTF16", err)
		}
		return
	}
	var schema []schemaRecord
	if err == nil {
		defer database.Close()
		schema, err = database.schemaRows()
	}
	if expected.err != "" {
		if err == nil {
			t.Fatalf("read without error; SQLite reports %q", expected.err)
		}
		return
	}
	if err != nil {
		t.Fatalf("open and read schema: %v", err)
	}
	var lines []string
	for _, row := range schema {
		lines = append(lines, describeSchema(row))
	}
	if !slices.Equal(lines, expected.schema) {
		t.Errorf("schema = %q, want %q", lines, expected.schema)
	}
	for _, tree := range expected.trees {
		checkTree(t, database, schema, tree)
	}
}

// checkTree compares one tree's rows with expected.
func checkTree(t *testing.T, database *Database, schema []schemaRecord, tree *expectedTree) {
	index := slices.IndexFunc(schema, func(row schemaRecord) bool { return row.Name == tree.name })
	if index < 0 {
		if !strings.HasPrefix(tree.err, "no such table") {
			t.Errorf("tree %s is not in the schema", tree.name)
		}
		return
	}
	rows, err := readTree(database, tree.kind, schema[index].RootPage)
	if tree.err != "" {
		if err == nil {
			t.Errorf("tree %s read without error; SQLite reports %q", tree.name, tree.err)
		}
		return
	}
	if err != nil {
		t.Errorf("tree %s: %v", tree.name, err)
	}
	if !slices.Equal(rows, tree.rows) {
		t.Errorf("tree %s: %d rows, want %d; first difference at %d", tree.name,
			len(rows), len(tree.rows), firstDifference(rows, tree.rows))
	}
}

// firstDifference returns the index of the first line where got and want differ.
func firstDifference(got, want []string) int {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return i
		}
	}
	return min(len(got), len(want))
}
