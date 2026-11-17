package parser

import (
	"errors"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

func TestOpenDatabaseNamesWhyADatabaseCannotBeRead(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name   string
		data   []byte
		prefix string
	}{
		{"a text file is not a database", []byte("not sqlite"), "not a SQLite database"},
		{"a header over a damaged body is a SQLite error",
			append([]byte("SQLite format 3\x00"), make([]byte, 200)...), "SQLite error: "},
		{"an empty file is an empty database", nil, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeFile(t, "codex-dev.db", test.data)
			database, message := OpenDatabase(path)
			if database != nil {
				defer database.Close()
			}
			if (test.prefix == "") != (message == "") || !strings.HasPrefix(message, test.prefix) {
				t.Errorf("got %q, want a message starting %q", message, test.prefix)
			}
		})
	}
	_, message := OpenDatabase(filepath.Join(directory, "absent.db"))
	if want := "absent.db could not be read (no such file or directory)"; message != want {
		t.Errorf("a missing file gave %q, want %q", message, want)
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

// The expected timestamps are what the Python analyzer's to_utc gives for
// the values its sqlite3 module returns.
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
