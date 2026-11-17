package parser

import (
	"errors"
	"io/fs"
	"math"

	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// OpenDatabase opens the SQLite database at path, with its write-ahead log
// and rollback journal, in place and read-only. When it cannot, it returns
// the message of the `parser:` row that says why: "not a SQLite database"
// for a file without the SQLite header, "<file name> could not be read
// (<reason>)" for the file or a sidecar that cannot be read, and
// "SQLite error: <reason>" for a database that cannot be opened.
func OpenDatabase(path string) (*sqlitefile.Database, string) {
	if err := sqlitefile.Problem(path); err != nil {
		return nil, DatabaseError(err)
	}
	database, err := sqlitefile.OpenPath(path)
	if err != nil {
		return nil, DatabaseError(err)
	}
	return database, ""
}

// DatabaseError returns the message of the `parser:` row for an error met
// while opening or reading a database: the unreadable message for a file
// that cannot be read, "not a SQLite database" for a file that is not one,
// and "SQLite error: <reason>" for anything else.
func DatabaseError(err error) string {
	var unreadable *sqlitefile.UnreadableError
	var pathError *fs.PathError
	switch {
	case errors.As(err, &unreadable):
		return reason.Unreadable(unreadable.Path, unreadable.Err)
	case errors.Is(err, sqlitefile.ErrNotDatabase):
		return sqlitefile.ErrNotDatabase.Error()
	case errors.As(err, &pathError):
		return reason.Unreadable(pathError.Path, err)
	}
	return "SQLite error: " + err.Error()
}

// ColumnTime returns the timestamp of a column value as Python's sqlite3
// module hands it to the time conversion: an integer or a float as an epoch
// number, text as a time or a digit string, and "" for NULL and a blob.
func ColumnTime(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		return timeutil.FromInt(value.Int)
	case sqlitefile.Float:
		return timeutil.FromEpoch(value.Float)
	case sqlitefile.Text:
		return timeutil.FromText(value.Text())
	}
	return ""
}

// ColumnText returns a column value as the text a parser writes into a row
// for a value that may be of any type: "" for NULL, text as it is, and
// anything else as the compact JSON of the value the sqlite3 module returns
// for it (an integer in decimal, a float as Python writes it, with
// Infinity and -Infinity for the infinities), a blob as its bytes literal.
func ColumnText(value sqlitefile.Value) string {
	switch {
	case value.IsNull():
		return ""
	case value.Type == sqlitefile.Float && math.IsInf(value.Float, 1):
		return "Infinity"
	case value.Type == sqlitefile.Float && math.IsInf(value.Float, -1):
		return "-Infinity"
	}
	return value.Str()
}
