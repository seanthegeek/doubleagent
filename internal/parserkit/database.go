package parserkit

import (
	"errors"
	"io/fs"
	"math"

	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// OpenDatabase opens the SQLite database at path, with its write-ahead log
// and rollback journal, in place and read-only. When it cannot, the error
// is sqlitefile.ErrNotDatabase for a main file that does not start with
// the SQLite header once its journal is played back, an
// *sqlitefile.UnreadableError for the file or a sidecar that cannot be
// read, or an *OpenError for a database that failed as it was opened;
// DatabaseError gives each as the text of the `parser:` row that says why.
func OpenDatabase(path string) (*sqlitefile.Database, error) {
	database, err := sqlitefile.OpenPath(path)
	var unreadable *sqlitefile.UnreadableError
	switch {
	case err == nil:
		return database, nil
	case errors.Is(err, sqlitefile.ErrNotDatabase):
		return nil, sqlitefile.ErrNotDatabase
	case errors.As(err, &unreadable):
		return nil, unreadable
	}
	return nil, &OpenError{Err: err}
}

// OpenError is the error of a database that is neither unreadable nor
// another kind of file but could not be opened, for a parser that words
// that case apart.
type OpenError struct {
	Err error
}

// Error returns the error opening met.
func (e *OpenError) Error() string { return e.Err.Error() }

// Unwrap returns the error opening met.
func (e *OpenError) Unwrap() error { return e.Err }

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

// ColumnTime returns the timestamp of a column value: an integer or a
// float as an epoch number, text as a time or a digit string (timeutil),
// and "" for NULL and a blob.
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
// for a value that may be of any type: its row text (sqlitefile.Value's
// String), with an infinite float as Infinity or -Infinity.
func ColumnText(value sqlitefile.Value) string {
	switch {
	case value.IsNull():
		return ""
	case value.Type == sqlitefile.Float && math.IsInf(value.Float, 1):
		return "Infinity"
	case value.Type == sqlitefile.Float && math.IsInf(value.Float, -1):
		return "-Infinity"
	}
	return value.String()
}
