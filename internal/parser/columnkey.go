package parser

import (
	"math"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// ColumnKey returns a column value as a map key that is equal exactly when
// two values are equal as SQLite compares them in a join: a number by its
// value whatever its storage class (an id stored as 3 and as 3.0 is one
// key), text and blobs by their bytes and kind, and NULL as itself.
func ColumnKey(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		return "n" + strconv.FormatInt(value.Int, 10)
	case sqlitefile.Float:
		if value.Float == math.Trunc(value.Float) && math.Abs(value.Float) < 1<<63 {
			return "n" + strconv.FormatInt(int64(value.Float), 10)
		}
		return "f" + strconv.FormatFloat(value.Float, 'g', -1, 64)
	case sqlitefile.Text:
		return "t" + value.Text()
	case sqlitefile.Blob:
		return "b" + value.Text()
	}
	return "null"
}
