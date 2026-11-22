package parser

import (
	"math"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// ColumnKey returns a column value as a map key that two values share
// exactly when SQL's = finds them equal with no affinity applied, as
// between two columns declared with no type: a number by its value
// whatever its storage class (3 and 3.0 are one key, as are -2^63 stored
// either way), text and blobs by their kind and bytes. NULL equals
// nothing, not even NULL, so it has no key: ColumnKey returns "", which no
// other value gives, and a caller joining on keys pairs no "" with "".
func ColumnKey(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		return "n" + strconv.FormatInt(value.Int, 10)
	case sqlitefile.Float:
		number := value.Float
		if number >= -1<<63 && number < 1<<63 && number == math.Trunc(number) {
			return "n" + strconv.FormatInt(int64(number), 10)
		}
		return "f" + strconv.FormatFloat(number, 'g', -1, 64)
	case sqlitefile.Text:
		return "t" + value.Text()
	case sqlitefile.Blob:
		return "b" + value.Text()
	}
	return ""
}

// CollatedKey returns ColumnKey of a value of a column that declares
// collation (sqlitefile.Column's Collation): text keys by its bytes with
// the ASCII capitals lowered under NOCASE and without its trailing spaces
// under RTRIM, as SQL's = compares two values of that column, and by its
// bytes under BINARY or any other name. Numbers, blobs and NULL key as
// ColumnKey keys them.
func CollatedKey(value sqlitefile.Value, collation string) string {
	if value.Type != sqlitefile.Text {
		return ColumnKey(value)
	}
	switch collation {
	case "NOCASE":
		return "t" + sqlitefile.FoldASCII(value.Text())
	case "RTRIM":
		return "t" + strings.TrimRight(value.Text(), " ")
	}
	return ColumnKey(value)
}

// KeyCollation returns the collation the column of table called name
// declares, its name matched in any ASCII case as SQL matches it; "" for
// BINARY, a column the table lacks or no table.
func KeyCollation(table *sqlitefile.Table, name string) string {
	if table == nil {
		return ""
	}
	for _, column := range table.Columns() {
		if sqlitefile.FoldASCII(column.Name) == sqlitefile.FoldASCII(name) {
			return column.Collation
		}
	}
	return ""
}
