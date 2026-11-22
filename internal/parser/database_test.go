package parser

import (
	"errors"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestOpenDatabaseNamesWhyADatabaseCannotBeRead(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name   string
		data   []byte
		prefix string
		opened bool
	}{
		{"a text file is not a database", []byte("not sqlite"), "not a SQLite database", false},
		{"a header over a damaged body is a SQLite error met opening it",
			append([]byte("SQLite format 3\x00"), make([]byte, 200)...), "SQLite error: ", true},
		{"an empty file is an empty database", nil, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeFile(t, "codex-dev.db", test.data)
			database, err := OpenDatabase(path)
			if database != nil {
				defer database.Close()
			}
			message := ""
			if err != nil {
				message = DatabaseError(err)
			}
			var openErr *OpenError
			if (test.prefix == "") != (err == nil) || !strings.HasPrefix(message, test.prefix) ||
				errors.As(err, &openErr) != test.opened {
				t.Errorf("got %v (%q), want a message starting %q, opening failed %v", err, message,
					test.prefix, test.opened)
			}
		})
	}
	_, err := OpenDatabase(filepath.Join(directory, "absent.db"))
	if want := "absent.db could not be read (no such file or directory)"; err == nil ||
		DatabaseError(err) != want {
		t.Errorf("a missing file gave %v, want %q", err, want)
	}
}

func TestDatabaseErrorWordsEachKindOfFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"an unreadable sidecar", &sqlitefile.UnreadableError{Path: "/x/state.db-wal",
			Err: fs.ErrPermission}, "state.db-wal could not be read (permission denied)"},
		{"a file read error", &fs.PathError{Op: "read", Path: "/x/state.db", Err: fs.ErrNotExist},
			"state.db could not be read (no such file or directory)"},
		{"not a database", sqlitefile.ErrNotDatabase, "not a SQLite database"},
		{"a damaged database", errors.Join(sqlitefile.ErrMalformed, errors.New("page 2")),
			"SQLite error: malformed database\npage 2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DatabaseError(test.err); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// Each storage class reads as timeutil reads a number or a text.
func TestColumnTimeReadsEachStorageClass(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  string
	}{
		{"an integer is epoch seconds", sqlitefile.IntValue(1790848800), "2026-10-01T10:00:00.000Z"},
		{"a float keeps its fraction", sqlitefile.FloatValue(1790852400.5), "2026-10-01T11:00:00.500Z"},
		{"text is a time", sqlitefile.TextValue("2026-10-01 10:00:00"), "2026-10-01T10:00:00.000Z"},
		{"NULL is no time", sqlitefile.Value{}, ""},
		{"a blob is no time", sqlitefile.BlobValue([]byte("1790848800")), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ColumnTime(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestColumnTextWritesEachStorageClassAsRowText(t *testing.T) {
	tests := []struct {
		name  string
		value sqlitefile.Value
		want  string
	}{
		{"NULL", sqlitefile.Value{}, ""},
		{"text", sqlitefile.TextValue("a b"), "a b"},
		{"empty text", sqlitefile.TextValue(""), ""},
		{"an integer", sqlitefile.IntValue(-7), "-7"},
		{"a whole float", sqlitefile.FloatValue(1), "1"},
		{"a small float", sqlitefile.FloatValue(0.00001), "1e-05"},
		{"positive infinity", sqlitefile.FloatValue(math.Inf(1)), "Infinity"},
		{"negative infinity", sqlitefile.FloatValue(math.Inf(-1)), "-Infinity"},
		{"a blob", sqlitefile.BlobValue([]byte("ab")), "x'6162'"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ColumnText(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// sharedMemoryCase is a state of the -shm file beside a database, made by
// place at the -shm file's path.
type sharedMemoryCase struct {
	name  string
	place func(t *testing.T, path string)
}

// checkSharedMemoryCases opens a copy of a database with each state of
// -shm beside it and fails unless the database opens and reads in full:
// SQLite rebuilds the -shm file from the log, so OpenDatabase never reads
// it.
func checkSharedMemoryCases(t *testing.T, cases []sharedMemoryCase) {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		t.Run("a database with "+test.name+" beside it opens and reads", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.vscdb")
			if err := osfile.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			test.place(t, path+"-shm")
			database, err := OpenDatabase(path)
			if err != nil {
				t.Fatalf("OpenDatabase() = %v, want the database open", err)
			}
			defer database.Close()
			table, err := database.Table("ItemTable")
			if err != nil {
				t.Fatal(err)
			}
			if rows, err := table.Select("key", "value").All(); err != nil || len(rows) == 0 {
				t.Errorf("Select().All() = %d rows, %v; want every row", len(rows), err)
			}
		})
	}
}

func TestADirectoryInPlaceOfTheSharedMemoryFileIsNoProblem(t *testing.T) {
	checkSharedMemoryCases(t, []sharedMemoryCase{{"a directory named as its -shm file",
		func(t *testing.T, path string) {
			if err := osfile.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}}})
}
